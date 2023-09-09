package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/dingding"
	"github.com/mylxsw/aidea-server/internal/mail"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/repo/model"
	"github.com/mylxsw/asteria/log"
)

type SignupPayload struct {
	ID         string    `json:"id,omitempty"`
	UserID     int64     `json:"user_id"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	InviteCode string    `json:"invite_code"`
	EventID    int64     `json:"event_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (payload *SignupPayload) GetTitle() string {
	return "用户注册"
}

func (payload *SignupPayload) SetID(id string) {
	payload.ID = id
}

func (payload *SignupPayload) GetID() string {
	return payload.ID
}

func (payload *SignupPayload) GetUID() int64 {
	return 0
}

func (payload *SignupPayload) GetQuotaID() int64 {
	return 0
}

func (payload *SignupPayload) GetQuota() int64 {
	return 0
}

func NewSignupTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeSignup, data)
}

func BuildSignupHandler(userRepo *repo.UserRepo, quotaRepo *repo.QuotaRepo, eventRepo *repo.EventRepo, queueRepo *repo.QueueRepo, roomRepo *repo.RoomRepo, mailer *mail.Sender, ding *dingding.Dingding) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload SignupPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		// 如果任务是 60 分钟前创建的，不再处理
		if payload.CreatedAt.Add(60 * time.Minute).Before(time.Now()) {
			return nil
		}

		defer func() {
			if err2 := recover(); err2 != nil {
				log.With(task).Errorf("panic: %v", err2)
				err = err2.(error)
			}

			if err != nil {
				if err := queueRepo.Update(
					context.TODO(),
					payload.GetID(),
					repo.QueueTaskStatusFailed,
					ErrorResult{
						Errors: []string{err.Error()},
					},
				); err != nil {
					log.With(task).Errorf("update queue status failed: %s", err)
				}

				if err := eventRepo.UpdateEvent(ctx, payload.EventID, repo.EventStatusFailed); err != nil {
					log.WithFields(log.Fields{"event_id": payload.EventID}).Errorf("update event status failed: %s", err)
				}
			}
		}()

		// 查询事件记录
		event, err := eventRepo.GetEvent(ctx, payload.EventID)
		if err != nil {
			if err == repo.ErrNotFound {
				log.WithFields(log.Fields{"event_id": payload.EventID}).Errorf("event not found")
				return nil
			}

			log.With(payload).Errorf("get event failed: %s", err)
			return err
		}

		if event.Status != repo.EventStatusWaiting {
			log.WithFields(log.Fields{"event_id": payload.EventID}).Warningf("event status is not waiting")
			return nil
		}

		if event.EventType != repo.EventTypeUserCreated {
			log.With(payload).Errorf("event type is not user_created")
			return nil
		}

		var eventPayload repo.UserCreatedEvent
		if err := json.Unmarshal([]byte(event.Payload), &eventPayload); err != nil {
			log.With(payload).Errorf("unmarshal event payload failed: %s", err)
			return err
		}

		// 为用户分配默认配额
		// 1. 如果是邮箱注册，不赠送智慧果，只有在用户绑定手机后才赠送
		// 2. 如果是手机注册，直接赠送智慧果
		if eventPayload.From == repo.UserCreatedEventSourceEmail {
			if coins.SignupGiftCoins > 0 {
				if _, err := quotaRepo.AddUserQuota(ctx, eventPayload.UserID, coins.SignupGiftCoins, time.Now().AddDate(0, 1, 0), "新用户注册赠送", ""); err != nil {
					log.WithFields(log.Fields{"user_id": eventPayload.UserID}).Errorf("create user quota failed: %s", err)
				}
			}
		} else if eventPayload.From == repo.UserCreatedEventSourcePhone {
			if _, err := quotaRepo.AddUserQuota(ctx, eventPayload.UserID, coins.BindPhoneGiftCoins, time.Now().AddDate(0, 1, 0), "新用户注册赠送", ""); err != nil {
				log.WithFields(log.Fields{"user_id": eventPayload.UserID}).Errorf("create user quota failed: %s", err)
			}
		}

		// 为用户生成自己的邀请码
		if err := userRepo.GenerateInviteCode(ctx, payload.UserID); err != nil {
			log.WithFields(log.Fields{"user_id": payload.UserID}).Errorf("生成邀请码失败: %s", err)
		}

		// 更新用户的邀请信息
		if payload.InviteCode != "" {
			inviteByUser, err := userRepo.GetUserByInviteCode(ctx, payload.InviteCode)
			if err != nil {
				if err != repo.ErrNotFound {
					log.With(payload).Errorf("通过邀请码查询用户失败: %s", err)
				}
			} else {
				if err := userRepo.UpdateUserInviteBy(ctx, eventPayload.UserID, inviteByUser.Id); err != nil {
					log.WithFields(log.Fields{"user_id": eventPayload.UserID, "invited_by": inviteByUser.Id}).Errorf("更新用户邀请信息失败: %s", err)
				} else {
					if eventPayload.From == repo.UserCreatedEventSourcePhone {
						// 为邀请人和被邀请人分配智慧果
						inviteGiftHandler(ctx, quotaRepo, eventPayload.UserID, inviteByUser.Id)
					}
				}
			}
		}

		// 为用户创建默认的数字人
		//createInitialRooms(ctx, roomRepo, eventPayload.UserID)

		// 更新事件状态
		if err := eventRepo.UpdateEvent(ctx, payload.EventID, repo.EventStatusSucceed); err != nil {
			log.WithFields(log.Fields{"event_id": payload.EventID}).Errorf("update event status failed: %s", err)
		}

		// 发送钉钉通知
		//go func() {
		//	content := fmt.Sprintf(
		//		`有新用户注册啦，账号 %s（ID：%d） 快去看看吧`,
		//		ternary.If(payload.Phone != "", payload.Phone, payload.Email),
		//		payload.UserID,
		//	)
		//	if err := ding.Send(dingding.NewMarkdownMessage("新用户注册啦", content, []string{})); err != nil {
		//		log.WithFields(log.Fields{"user_id": eventPayload.UserID}).Errorf("send dingding message failed: %s", err)
		//	}
		//}()

		return queueRepo.Update(
			context.TODO(),
			payload.GetID(),
			repo.QueueTaskStatusSuccess,
			EmptyResult{},
		)
	}
}

type InitRoom struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	Vendor     string `json:"vendor"`
	Prompt     string `json:"prompt"`
	MaxContext int64  `json:"max_context"`
}

// 为用户创建默认的数字人
func createInitialRooms(ctx context.Context, roomRepo *repo.RoomRepo, userID int64) {
	items, err := roomRepo.Galleries(ctx)
	if err != nil {
		log.WithFields(log.Fields{"user_id": userID}).Errorf("获取数字人列表失败: %s", err)
		return
	}

	for _, item := range items {
		if _, err := roomRepo.Create(ctx, userID, &model.Rooms{
			Name:           item.Name,
			Model:          item.Model,
			Vendor:         item.Vendor,
			SystemPrompt:   item.Prompt,
			MaxContext:     item.MaxContext,
			RoomType:       repo.RoomTypePreset,
			InitMessage:    item.InitMessage,
			AvatarId:       item.AvatarId,
			AvatarUrl:      item.AvatarUrl,
			LastActiveTime: time.Now(),
		}, false); err != nil {
			log.WithFields(log.Fields{
				"room":    item,
				"user_id": userID,
			}).Errorf("用户注册后创建默认数字人失败: %s", err)
		}
	}
}

func inviteGiftHandler(ctx context.Context, quotaRepo *repo.QuotaRepo, userId, invitedByUserId int64) {
	// 引荐人奖励
	if coins.InviteGiftCoins > 0 {
		if _, err := quotaRepo.AddUserQuota(ctx, invitedByUserId, coins.InviteGiftCoins, time.Now().AddDate(0, 1, 0), "引荐奖励", ""); err != nil {
			log.WithFields(log.Fields{"user_id": invitedByUserId}).Errorf("create user quota failed: %s", err)
		}
	}

	// 被引荐人奖励
	if coins.InvitedGiftCoins > 0 {
		if _, err := quotaRepo.AddUserQuota(ctx, userId, coins.InvitedGiftCoins, time.Now().AddDate(0, 1, 0), "引荐注册奖励", ""); err != nil {
			log.WithFields(log.Fields{"user_id": userId}).Errorf("create user quota failed: %s", err)
		}
	}
}
