package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/mail"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/asteria/log"
)

type MailPayload struct {
	ID        string    `json:"id,omitempty"`
	To        []string  `json:"to"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

func (payload *MailPayload) GetTitle() string {
	return payload.Subject
}

func (payload *MailPayload) SetID(id string) {
	payload.ID = id
}

func (payload *MailPayload) GetID() string {
	return payload.ID
}

func (payload *MailPayload) GetUID() int64 {
	return 0
}

func (payload *MailPayload) GetQuotaID() int64 {
	return 0
}

func (payload *MailPayload) GetQuota() int64 {
	return 0
}

func NewMailTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeMailSend, data)
}

func BuildMailSendHandler(mailer *mail.Sender, rep *repo2.Repository) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload MailPayload
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

		if err := mailer.Send(payload.To, fmt.Sprintf("【AIdea】%s", payload.Subject), payload.Body); err != nil {
			log.With(payload).Errorf("send mail failed: %v", err)
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
