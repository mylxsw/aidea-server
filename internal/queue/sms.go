package queue

import (
	"context"
	"encoding/json"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/sms"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/asteria/log"
)

type SMSVerifyCodePayload struct {
	ID        string    `json:"id,omitempty"`
	Receiver  string    `json:"receiver"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
}

func (payload *SMSVerifyCodePayload) GetTitle() string {
	return "短信验证码"
}

func (payload *SMSVerifyCodePayload) SetID(id string) {
	payload.ID = id
}

func (payload *SMSVerifyCodePayload) GetID() string {
	return payload.ID
}

func (payload *SMSVerifyCodePayload) GetUID() int64 {
	return 0
}

func (payload *SMSVerifyCodePayload) GetQuotaID() int64 {
	return 0
}

func (payload *SMSVerifyCodePayload) GetQuota() int64 {
	return 0
}

func NewSMSVerifyCodeTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeSMSVerifyCodeSend, data)
}

func BuildSMSVerifyCodeSendHandler(sender *sms.Client, rep *repo2.Repository) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload SMSVerifyCodePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		// 如果任务是 5 分钟前创建的，不再处理
		if payload.CreatedAt.Add(5 * time.Minute).Before(time.Now()) {
			return nil
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
					repo2.QueueTaskStatusFailed,
					ErrorResult{
						Errors: []string{err.Error()},
					},
				); err != nil {
					log.With(task).Errorf("update queue status failed: %s", err)
				}
			}
		}()

		if err := sender.SendVerifyCode(ctx, payload.Code, payload.Receiver); err != nil {
			log.With(payload).Errorf("send sms verify code failed: %v", err)
			return err
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo2.QueueTaskStatusSuccess,
			EmptyResult{},
		)
	}
}
