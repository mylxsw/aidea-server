package queue

import (
	"context"
	"encoding/json"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
)

type OpenAICompletionPayload struct {
	ID           string                         `json:"id,omitempty"`
	Model        string                         `json:"model,omitempty"`
	Quota        int64                          `json:"quota,omitempty"`
	UID          int64                          `json:"uid,omitempty"`
	Prompts      []openai.ChatCompletionMessage `json:"prompts,omitempty"`
	WordCount    int64                          `json:"word_count,omitempty"`
	CreatedAt    time.Time                      `json:"created_at,omitempty"`
	FreezedCoins int64                          `json:"freezed_coins,omitempty"`
}

func (payload *OpenAICompletionPayload) GetTitle() string {
	m := []rune(payload.Prompts[len(payload.Prompts)-1].Content)
	if len(m) > 50 {
		return string(m[:50]) + "..."
	}

	return string(m)
}

func (payload *OpenAICompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *OpenAICompletionPayload) GetID() string {
	return payload.ID
}

func (payload *OpenAICompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *OpenAICompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func NewOpenAICompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeOpenAICompletion, data)
}

func BuildOpenAICompletionHandler(client openai2.Client, rep *repo2.Repository) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload OpenAICompletionPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		// 如果任务是 15 分钟前创建的，不再处理
		if payload.CreatedAt.Add(15 * time.Minute).Before(time.Now()) {
			return nil
		}

		if err := rep.Queue.Update(context.TODO(), payload.GetID(), repo2.QueueTaskStatusRunning, nil); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("set task status to running failed: %s", err)
			return err
		}

		if payload.CreatedAt.Add(15 * time.Minute).Before(time.Now()) {
			rep.Queue.Update(context.TODO(), payload.GetID(), repo2.QueueTaskStatusFailed, ErrorResult{Errors: []string{"任务处理超时"}})
			log.WithFields(log.Fields{"payload": payload}).Errorf("task expired")
			return nil
		}

		defer func() {
			if err2 := recover(); err2 != nil {
				log.With(task).Errorf("panic: %v", err2)
				err = err2.(error)

				// 更新创作岛历史记录
				if err := rep.Creative.UpdateRecordByTaskID(ctx, payload.GetUID(), payload.GetID(), repo2.CreativeRecordUpdateRequest{
					Status: repo2.CreativeStatusFailed,
					Answer: err.Error(),
				}); err != nil {
					log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
				}
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

		contextTokenCount, err := openai2.NumTokensFromMessages(payload.Prompts, payload.Model)
		if err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("get context token count failed: %s", err)
			return err
		}

		req := openai.ChatCompletionRequest{
			Model:       openai2.SelectBestModel(payload.Model, contextTokenCount),
			MaxTokens:   int(payload.WordCount) * 2, // 假设一个汉字 = 2 token
			Temperature: 1,
			Messages:    payload.Prompts,
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			log.WithFields(log.Fields{"req": req}).Errorf("create completion failed: %v", err)
			// 此处故意 panic， 进入 defer 逻辑，以便更新 creative 状态
			panic(err)
		}

		log.F(log.M{"req": req, "resp": resp}).Debugf("create completion success")

		// 修改为实际消耗的 Token 数量
		if resp.Usage.TotalTokens > 0 {
			payload.Quota = coins.GetOpenAITextCoins(payload.Model, int64(resp.Usage.TotalTokens))
		}

		content := array.Reduce(
			resp.Choices,
			func(carry string, item openai.ChatCompletionChoice) string {
				return carry + "\n" + item.Message.Content
			},
			"",
		)

		// 更新创作岛历史记录
		updateReq := repo2.CreativeRecordUpdateRequest{
			Status:    repo2.CreativeStatusSuccess,
			Answer:    content,
			QuotaUsed: payload.GetQuota(),
		}
		if err := rep.Creative.UpdateRecordByTaskID(ctx, payload.GetUID(), payload.GetID(), updateReq); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			return err
		}

		// 记录消耗
		if err := rep.Quota.QuotaConsume(ctx, payload.UID, payload.Quota, repo2.NewQuotaUsedMeta("openai", payload.Model)); err != nil {
			log.With(payload).Errorf("used quota add failed: %s", err)
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo2.QueueTaskStatusSuccess,
			CompletionResult{Resources: []string{content}},
		)
	}
}
