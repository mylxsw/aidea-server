package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/service"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/ternary"
)

type GroupChatPayload struct {
	ID              string        `json:"id,omitempty"`
	GroupID         int64         `json:"group_id,omitempty"`
	UserID          int64         `json:"user_id,omitempty"`
	MemberID        int64         `json:"member_id,omitempty"`
	QuestionID      int64         `json:"question_id,omitempty"`
	MessageID       int64         `json:"message_id,omitempty"`
	ModelID         string        `json:"model_id,omitempty"`
	ContextMessages chat.Messages `json:"context_messages,omitempty"`
	CreatedAt       time.Time     `json:"created_at,omitempty"`
}

func (payload *GroupChatPayload) GetTitle() string {
	return "群聊"
}

func (payload *GroupChatPayload) SetID(id string) {
	payload.ID = id
}

func (payload *GroupChatPayload) GetID() string {
	return payload.ID
}

func (payload *GroupChatPayload) GetUID() int64 {
	return payload.UserID
}

func (payload *GroupChatPayload) GetQuotaID() int64 {
	return 0
}

func (payload *GroupChatPayload) GetQuota() int64 {
	return 0
}

func NewGroupChatTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeGroupChat, data)
}

func BuildGroupChatHandler(conf *config.Config, ct chat.Chat, rep *repo.Repository, userSrv *service.UserService) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload GroupChatPayload
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
				// 更新消息状态为失败
				msg := repo.ChatGroupMessageUpdate{
					Message: err.Error(),
					Status:  repo.MessageStatusFailed,
				}
				if err := rep.ChatGroup.UpdateChatMessage(ctx, payload.GroupID, payload.UserID, payload.MessageID, msg); err != nil {
					log.With(task).Errorf("update chat message failed: %s", err)
				}

				// 更新队列状态为失败
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
			}
		}()

		chatReq, err := (chat.Request{
			Model:    payload.ModelID,
			Messages: payload.ContextMessages,
		}).Fix(ct, 5)
		if err != nil {
			panic(fmt.Errorf("fix chat request failed: %w", err))
		}

		// 调用 AI 系统
		resp, err := ct.Chat(ctx, chatReq.Request)
		if err != nil {
			panic(fmt.Errorf("chat failed: %w", err))
		}

		if resp.ErrorCode != "" {
			panic(fmt.Errorf("chat failed: %s %s", resp.ErrorCode, resp.Error))
		}

		tokenConsumed := int64(resp.InputTokens + resp.OutputTokens)
		// 免费请求不计费
		leftCount, _ := userSrv.FreeChatRequestCounts(ctx, payload.UserID, chatReq.Request.Model)
		quotaConsumed := ternary.IfLazy(
			leftCount > 0,
			func() int64 { return 0 },
			func() int64 { return coins.GetOpenAITextCoins(chatReq.Request.ResolveCalFeeModel(conf), tokenConsumed) },
		)

		// 更新消息状态
		msg := repo.ChatGroupMessageUpdate{
			Message:       resp.Text,
			TokenConsumed: tokenConsumed,
			QuotaConsumed: quotaConsumed,
			Status:        repo.MessageStatusSucceed,
		}
		if err := rep.ChatGroup.UpdateChatMessage(ctx, payload.GroupID, payload.UserID, payload.MessageID, msg); err != nil {
			panic(fmt.Errorf("update chat message failed: %w", err))
		}

		// 更新免费聊天次数
		if err := userSrv.UpdateFreeChatCount(ctx, payload.UserID, chatReq.Request.Model); err != nil {
			log.With(payload).Errorf("update free chat count failed: %s", err)
		}

		// 扣除智慧果
		if quotaConsumed > 0 {
			if err := rep.Quota.QuotaConsume(ctx, payload.UserID, quotaConsumed, repo.NewQuotaUsedMeta("group_chat", chatReq.Request.Model)); err != nil {
				log.Errorf("used quota add failed: %s", err)
			}
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo.QueueTaskStatusSuccess,
			EmptyResult{},
		)
	}
}
