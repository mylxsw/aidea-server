package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/dingding"
	"github.com/mylxsw/aidea-server/internal/mail"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/asteria/log"
)

type PaymentPayload struct {
	ID        string    `json:"id,omitempty"`
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	ProductID string    `json:"product_id"`
	PaymentID string    `json:"payment_id"`
	Note      string    `json:"note"`
	Source    string    `json:"source"`
	Env       string    `json:"env"`
	CreatedAt time.Time `json:"created_at"`
	EventID   int64     `json:"event_id"`
}

func (payload *PaymentPayload) GetTitle() string {
	return payload.Source
}

func (payload *PaymentPayload) SetID(id string) {
	payload.ID = id
}

func (payload *PaymentPayload) GetID() string {
	return payload.ID
}

func (payload *PaymentPayload) GetUID() int64 {
	return 0
}

func (payload *PaymentPayload) GetQuotaID() int64 {
	return 0
}

func (payload *PaymentPayload) GetQuota() int64 {
	return 0
}

func NewPaymentTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypePayment, data)
}

func BuildPaymentHandler(
	rep *repo.Repository,
	mailer *mail.Sender,
	que *Queue,
	ding *dingding.Dingding,
) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload PaymentPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		defer func() {
			if err2 := recover(); err2 != nil {
				log.With(task).Errorf("panic: %v", err2)
				err = err2.(error)
			}

			if err != nil {
				if err := rep.Queue.Update(
					context.TODO(),
					payload.GetID(),
					repo.QueueTaskStatusFailed,
					ErrorResult{
						Errors: []string{err.Error()},
					},
				); err != nil {
					log.With(task).Errorf("update queue status failed: %s", err)
				}

				if err := rep.Event.UpdateEvent(ctx, payload.EventID, repo.EventStatusFailed); err != nil {
					log.WithFields(log.Fields{"event_id": payload.EventID}).Errorf("update event status failed: %s", err)
				}
			}
		}()

		// 查询事件记录
		event, err := rep.Event.GetEvent(ctx, payload.EventID)
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

		if event.EventType != repo.EventTypePaymentCompleted {
			log.With(payload).Errorf("event type is not payment_completed")
			return nil
		}

		// 用户充值
		product := coins.GetProduct(payload.ProductID)
		if product == nil {
			log.With(payload).Errorf("product not found")
			return nil
		}

		expiredAt := product.ExpiredAt()
		if _, err := rep.Quota.AddUserQuota(ctx, payload.UserID, product.Quota, expiredAt, payload.Note, payload.PaymentID); err != nil {
			log.With(payload).Errorf("用户充值增加配额失败: %s", err)
			return err
		}

		if err := rep.Event.UpdateEvent(ctx, payload.EventID, repo.EventStatusSucceed); err != nil {
			log.WithFields(log.Fields{"event_id": payload.EventID}).Errorf("update event status failed: %s", err)
		}

		// 如果用户配置了邮箱，则发送邮件通知
		if payload.Email != "" {
			mailPayload := &MailPayload{
				To:        []string{payload.Email},
				Subject:   "充值已到账",
				Body:      fmt.Sprintf("您充值的 %d 个智慧果已到账，有效期至 %s，请尽快使用。", product.Quota, repo.TimeInDate(expiredAt).Format(time.RFC3339)),
				CreatedAt: time.Now(),
			}

			if _, err := que.Enqueue(mailPayload, NewMailTask, asynq.Queue("mail")); err != nil {
				log.With(mailPayload).Errorf("failed to enqueue mail task: %s", err)
			}
		}

		// 邀请人奖励
		user, err := rep.User.GetUserByID(ctx, payload.UserID)
		if err != nil {
			if err != repo.ErrUserAccountDisabled {
				log.WithFields(log.Fields{"user_id": payload.UserID}).Errorf("引荐人奖励，查询用户信息失败: %s", err)
			}
		} else {
			// 有引荐人的时候，每次充值，都会增加引荐人的奖励
			// 有效期为一年内
			if user.InvitedBy > 0 && user.CreatedAt.After(time.Now().AddDate(-1, 0, 0)) {
				// 为邀请人增加奖励
				if _, err := rep.Quota.AddUserQuota(ctx, user.InvitedBy, int64(coins.InvitePaymentGiftRate*float64(product.Quota)), time.Now().AddDate(0, 1, 0), "引荐人充值分红", payload.PaymentID); err != nil {
					log.WithFields(log.Fields{"user_id": user.InvitedBy}).Errorf("引荐人充值分红失败: %s", err)
				}
			}
		}

		// 发送钉钉通知
		go func() {
			content := fmt.Sprintf(
				`用户（ID：%d）充值了 %d 个智慧果，有效期至 %s，充值订单号为 %s，充值来源为 %s。`,
				payload.UserID,
				product.Quota,
				repo.TimeInDate(expiredAt).Format("2006-01-02"),
				payload.PaymentID,
				payload.Source,
			)
			if err := ding.Send(dingding.NewMarkdownMessage(payload.Env+": 有用户充值啦", content, []string{})); err != nil {
				log.Errorf("发送钉钉通知失败: %s", err)
			}
		}()

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo.QueueTaskStatusSuccess,
			EmptyResult{},
		)
	}
}
