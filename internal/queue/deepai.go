package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/ai/deepai"
	"github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
)

type DeepAICompletionPayload struct {
	ID             string    `json:"id,omitempty"`
	Model          string    `json:"model,omitempty"`
	Quota          int64     `json:"quota,omitempty"`
	UID            int64     `json:"uid,omitempty"`
	Prompt         string    `json:"prompt,omitempty"`
	PromptTags     []string  `json:"prompt_tags,omitempty"`
	NegativePrompt string    `json:"negative_prompt,omitempty"`
	Width          int64     `json:"width,omitempty"`
	Height         int64     `json:"height,omitempty"`
	ImageCount     int64     `json:"grid_size,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	FilterID       int64     `json:"filter_id,omitempty"`

	AIRewrite bool `json:"ai_rewrite,omitempty"`
}

func (payload *DeepAICompletionPayload) GetTitle() string {
	return payload.Prompt
}

func (payload *DeepAICompletionPayload) GetID() string {
	return payload.ID
}

func (payload *DeepAICompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *DeepAICompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *DeepAICompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func NewDeepAICompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeDeepAICompletion, data)
}

func BuildDeepAICompletionHandler(client *deepai.DeepAI, translator youdao.Translater, up *uploader.Uploader, rep *repo.Repository, oai *openai.OpenAI) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload DeepAICompletionPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		if err := rep.Queue.Update(context.TODO(), payload.GetID(), repo.QueueTaskStatusRunning, nil); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("set task status to running failed: %s", err)
			return err
		}

		if payload.CreatedAt.Add(5 * time.Minute).Before(time.Now()) {
			rep.Queue.Update(context.TODO(), payload.GetID(), repo.QueueTaskStatusFailed, ErrorResult{Errors: []string{"任务处理超时"}})
			log.WithFields(log.Fields{"payload": payload}).Errorf("task expired")
			return nil
		}

		defer func() {
			if err2 := recover(); err2 != nil {
				log.With(task).Errorf("panic: %v", err2)
				err = err2.(error)

				// 更新创作岛历史记录
				if err := rep.Creative.UpdateRecordByTaskID(ctx, payload.GetUID(), payload.GetID(), repo.CreativeRecordUpdateRequest{
					Status: repo.CreativeStatusFailed,
					Answer: err.Error(),
				}); err != nil {
					log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
				}
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
			}
		}()

		var prompt, negativePrompt string
		prompt, negativePrompt, payload.AIRewrite = resolvePrompts(
			ctx,
			PromptResolverPayload{
				Prompt:         payload.Prompt,
				PromptTags:     payload.PromptTags,
				NegativePrompt: payload.NegativePrompt,
				FilterID:       payload.FilterID,
				AIRewrite:      payload.AIRewrite,
				Vendor:         "deepai",
				Model:          payload.Model,
			},
			rep.Creative,
			oai, translator,
		)

		res, err := client.TextToImage(payload.Model, deepai.TextToImageParam{
			Text:         prompt,
			Width:        int(payload.Width),
			Height:       int(payload.Height),
			GridSize:     int(payload.ImageCount),
			NegativeText: negativePrompt,
		})
		if err != nil {
			log.With(payload).Errorf("create completion failed: %v", err)
			// 此处故意 panic， 进入 defer 逻辑，以便更新 creative 状态
			panic(err)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// 上传图片到七牛云
		tmpURL, err := up.UploadRemoteFile(ctx, res.OutputURL, int(payload.GetUID()), uploader.DefaultUploadExpireAfterDays, "png", false)
		if err != nil {
			log.With(payload).Errorf("upload image to qiniu failed: %v", err)
		} else {
			res.OutputURL = tmpURL
		}

		modelUsed := []string{payload.Model, "upload"}

		ret := []string{res.OutputURL}
		if len(ret) == 0 {
			log.WithFields(log.Fields{
				"payload": payload,
			}).Errorf("没有生成任何图片")
			panic(errors.New("没有生成任何图片"))
		}

		// 更新创作岛历史记录
		retJson, err := json.Marshal(ret)
		if err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			panic(err)
		}

		req := repo.CreativeRecordUpdateRequest{
			Status:    repo.CreativeStatusSuccess,
			Answer:    string(retJson),
			QuotaUsed: payload.GetQuota(),
		}

		if prompt != payload.Prompt || negativePrompt != payload.NegativePrompt {
			ext := repo.CreativeRecordUpdateExtArgs{}
			if prompt != payload.Prompt {
				ext.RealPrompt = prompt
			}

			if negativePrompt != payload.NegativePrompt {
				ext.RealNegativePrompt = negativePrompt
			}

			req.ExtArguments = &ext
		}

		if err := rep.Creative.UpdateRecordByTaskID(ctx, payload.GetUID(), payload.GetID(), req); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			return err
		}

		// 更新用户配额
		if err := rep.Quota.QuotaConsume(
			ctx,
			payload.GetUID(),
			payload.GetQuota(),
			repo.NewQuotaUsedMeta("deepai", modelUsed...),
		); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo.QueueTaskStatusSuccess,
			CompletionResult{
				Resources:   ret,
				ValidBefore: time.Now().Add(7 * 24 * time.Hour),
			},
		)
	}
}
