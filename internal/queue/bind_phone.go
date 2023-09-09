package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/mail"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/asteria/log"
)

type BindPhonePayload struct {
	ID         string    `json:"id,omitempty"`
	UserID     int64     `json:"user_id"`
	Phone      string    `json:"phone"`
	InviteCode string    `json:"invite_code"`
	EventID    int64     `json:"event_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (payload *BindPhonePayload) GetTitle() string {
	return "手机绑定"
}

func (payload *BindPhonePayload) SetID(id string) {
	payload.ID = id
}

func (payload *BindPhonePayload) GetID() string {
	return payload.ID
}

func (payload *BindPhonePayload) GetUID() int64 {
	return 0
}

func (payload *BindPhonePayload) GetQuotaID() int64 {
	return 0
}

func (payload *BindPhonePayload) GetQuota() int64 {
	return 0
}

func NewBindPhoneTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeBindPhone, data)
}

func BuildBindPhoneHandler(userRepo *repo.UserRepo, quotaRepo *repo.QuotaRepo, eventRepo *repo.EventRepo, queueRepo *repo.QueueRepo, mailer *mail.Sender) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload BindPhonePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		// 如果任务是 15 分钟前创建的，不再处理
		if payload.CreatedAt.Add(15 * time.Minute).Before(time.Now()) {
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

		if event.EventType != repo.EventTypeUserPhoneBound {
			log.With(payload).Errorf("event type is not user_phone_bound")
			return nil
		}

		var eventPayload repo.UserBindEvent
		if err := json.Unmarshal([]byte(event.Payload), &eventPayload); err != nil {
			log.With(payload).Errorf("unmarshal event payload failed: %s", err)
			return err
		}

		// 为用户分配默认配额
		if coins.BindPhoneGiftCoins > 0 {
			if _, err := quotaRepo.AddUserQuota(ctx, eventPayload.UserID, coins.BindPhoneGiftCoins, time.Now().AddDate(0, 1, 0), "绑定手机赠送", ""); err != nil {
				log.WithFields(log.Fields{"user_id": eventPayload.UserID}).Errorf("create user quota failed: %s", err)
			}
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
					// 为邀请人和被邀请人分配智慧果
					inviteGiftHandler(ctx, quotaRepo, eventPayload.UserID, inviteByUser.Id)
				}
			}
		}

		// 更新事件状态
		if err := eventRepo.UpdateEvent(ctx, payload.EventID, repo.EventStatusSucceed); err != nil {
			log.WithFields(log.Fields{"event_id": payload.EventID}).Errorf("update event status failed: %s", err)
		}

		return queueRepo.Update(
			context.TODO(),
			payload.GetID(),
			repo.QueueTaskStatusSuccess,
			EmptyResult{},
		)
	}
}
