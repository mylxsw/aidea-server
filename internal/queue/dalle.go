package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/asteria/log"
	"strings"
	"time"
)

type DalleCompletionPayload struct {
	ID    string `json:"id,omitempty"`
	Model string `json:"model,omitempty"`
	Quota int64  `json:"quota,omitempty"`
	UID   int64  `json:"uid,omitempty"`

	Prompt      string   `json:"prompt,omitempty"`
	PromptTags  []string `json:"prompt_tags,omitempty"`
	ImageCount  int64    `json:"image_count,omitempty"`
	Width       int64    `json:"width,omitempty"`
	Height      int64    `json:"height,omitempty"`
	StylePreset string   `json:"style_preset,omitempty"`
	FilterID    int64    `json:"filter_id,omitempty"`

	CreatedAt    time.Time `json:"created_at,omitempty"`
	FreezedCoins int64     `json:"freezed_coins,omitempty"`
}

func (payload *DalleCompletionPayload) GetTitle() string {
	return payload.Prompt
}

func (payload *DalleCompletionPayload) GetID() string {
	return payload.ID
}

func (payload *DalleCompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *DalleCompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *DalleCompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func NewDalleCompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeDalleCompletion, data)
}

func BuildDalleCompletionHandler(client *openai.DalleImageClient, up *uploader.Uploader, rep *repo2.Repository) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload DalleCompletionPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		if payload.CreatedAt.Add(5 * time.Minute).Before(time.Now()) {
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

		// 下载远程图片（图生图）
		var prompt string
		prompt, _, _ = resolvePrompts(
			ctx,
			PromptResolverPayload{
				Prompt:     payload.Prompt,
				PromptTags: payload.PromptTags,
				FilterID:   payload.FilterID,
				AIRewrite:  false,
				Vendor:     "dalle",
				Model:      payload.Model,
			},
			rep.Creative,
			nil, nil,
		)

		// 模型名称格式：
		// dall-e-3    -> model: dalle-e-3 quality: standard
		// dall-e-3:hd -> model: dalle-e-3 quality: hd
		var model, quality string
		modelSegs := strings.SplitN(payload.Model, ":", 2)
		if len(modelSegs) != 2 {
			model = payload.Model
		} else {
			model = modelSegs[0]
			quality = modelSegs[1]
		}

		resp, err := client.CreateImage(ctx, openai.ImageRequest{
			Prompt:         prompt,
			Model:          model,
			N:              payload.ImageCount,
			Size:           fmt.Sprintf("%dx%d", payload.Width, payload.Height),
			Style:          payload.StylePreset,
			Quality:        quality,
			ResponseFormat: "b64_json",
		})

		if err != nil {
			log.With(payload).Errorf("[Dalle] 图片生成失败: %v", err)
			panic(err)
		}

		resources, err := resp.UploadResources(ctx, up, payload.GetUID())
		if err != nil {
			log.WithFields(log.Fields{
				"payload": payload,
			}).Errorf(err.Error())
			panic(err)
		}

		if len(resources) == 0 {
			log.WithFields(log.Fields{
				"payload": payload,
			}).Errorf("没有生成任何图片")
			panic(errors.New("没有生成任何图片"))
		}

		modelUsed := []string{payload.Model, "upload"}

		// 更新创作岛历史记录

		retJson, err := json.Marshal(resources)
		if err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			panic(err)
		}

		updateReq := repo2.CreativeRecordUpdateRequest{
			Status:    repo2.CreativeStatusSuccess,
			Answer:    string(retJson),
			QuotaUsed: payload.GetQuota(),
		}

		if prompt != payload.Prompt {
			ext := repo2.CreativeRecordUpdateExtArgs{}
			if prompt != payload.Prompt {
				ext.RealPrompt = prompt
			}

			updateReq.ExtArguments = &ext
		}

		if err := rep.Creative.UpdateRecordByTaskID(ctx, payload.GetUID(), payload.GetID(), updateReq); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			return err
		}

		if err := rep.Quota.QuotaConsume(
			ctx,
			payload.GetUID(),
			payload.GetQuota(),
			repo2.NewQuotaUsedMeta("dalle", modelUsed...),
		); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo2.QueueTaskStatusSuccess,
			CompletionResult{
				Resources:   resources,
				ValidBefore: time.Now().Add(7 * 24 * time.Hour),
			},
		)
	}
}
