package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dashscope2 "github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
)

type DashscopeImageCompletionPayload struct {
	ID    string `json:"id,omitempty"`
	Model string `json:"model,omitempty"`
	Quota int64  `json:"quota,omitempty"`
	UID   int64  `json:"uid,omitempty"`

	Prompt     string   `json:"prompt,omitempty"`
	PromptTags []string `json:"prompt_tags,omitempty"`

	NegativePrompt string `json:"negative_prompt,omitempty"`
	ImageCount     int64  `json:"image_count,omitempty"`
	Width          int64  `json:"width,omitempty"`
	Height         int64  `json:"height,omitempty"`
	Steps          int64  `json:"steps,omitempty"`
	Seed           int64  `json:"seed,omitempty"`

	Image         string  `json:"image,omitempty"`
	AIRewrite     bool    `json:"ai_rewrite,omitempty"`
	ImageStrength float64 `json:"image_strength,omitempty"`
	FilterID      int64   `json:"filter_id,omitempty"`

	CreatedAt    time.Time `json:"created_at,omitempty"`
	FreezedCoins int64     `json:"freezed_coins,omitempty"`
}

func (payload *DashscopeImageCompletionPayload) GetTitle() string {
	return payload.Prompt
}

func (payload *DashscopeImageCompletionPayload) GetID() string {
	return payload.ID
}

func (payload *DashscopeImageCompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *DashscopeImageCompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *DashscopeImageCompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func (payload *DashscopeImageCompletionPayload) GetModel() string {
	return payload.Model
}

func NewDashscopeImageCompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeDashscopeImageCompletion, data)
}

type DashscopeImagePendingTaskPayload struct {
	DashscopeImageTaskID string                          `json:"dashscope_image_task_id,omitempty"`
	Payload              DashscopeImageCompletionPayload `json:"payload,omitempty"`
}

func (p DashscopeImagePendingTaskPayload) GetImage() string {
	return p.Payload.Image
}

func (p DashscopeImagePendingTaskPayload) GetID() string {
	return p.Payload.GetID()
}

func (p DashscopeImagePendingTaskPayload) GetUID() int64 {
	return p.Payload.UID
}

func (p DashscopeImagePendingTaskPayload) GetQuota() int64 {
	return p.Payload.Quota
}

func (p DashscopeImagePendingTaskPayload) GetModel() string {
	return p.Payload.Model
}

type DashscopeImageResponse interface {
	GetID() string
	GetState() string
	IsFinished() bool
	IsProcessing() bool
	UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) ([]string, error)
	GetImages() []string
}

func BuildDashscopeImageCompletionHandler(
	client *dashscope2.DashScope,
	up *uploader.Uploader,
	rep *repo2.Repository,
	translator youdao.Translater,
	oai openai.Client,
) TaskHandler {

	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload DashscopeImageCompletionPayload
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
					Answer: err.Error(),
					Status: repo2.CreativeStatusFailed,
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

		var prompt, negativePrompt string
		prompt, negativePrompt, payload.AIRewrite = resolvePrompts(
			ctx,
			PromptResolverPayload{
				Prompt:         payload.Prompt,
				PromptTags:     payload.PromptTags,
				NegativePrompt: payload.NegativePrompt,
				FilterID:       payload.FilterID,
				AIRewrite:      payload.AIRewrite,
				Image:          payload.Image,
				Vendor:         "dashscope",
				Model:          payload.Model,
			},
			rep.Creative,
			oai,
			translator,
		)

		var resp *dashscope2.ImageGenerationResponse
		if payload.Image != "" {
			// 图生图模式，调用人像风格重绘接口
			req := dashscope2.ImageGenerationRequest{
				Model: payload.GetModel(),
				Input: dashscope2.ImageGenerationRequestInput{
					ImageURL:   payload.Image,
					StyleIndex: dashscope2.ImageStyleComic,
				},
			}

			resp, err = client.ImageGeneration(ctx, req)
			if err != nil {
				log.With(payload).Errorf("create completion failed: %v", err)
				panic(err)
			}

		} else {
			// 文生图模式，调用 Stable Diffusion 接口
			req := dashscope2.StableDiffusionRequest{
				Model: payload.GetModel(),
				Input: dashscope2.StableDiffusionInput{
					Prompt:         prompt,
					NegativePrompt: negativePrompt,
				},
				Parameters: dashscope2.StableDiffusionParameters{
					Size:  fmt.Sprintf("%d*%d", payload.Width, payload.Height),
					N:     int(payload.ImageCount),
					Steps: int(payload.Steps),
					Seed:  int(payload.Seed),
				},
			}

			resp, err = client.StableDiffusion(ctx, req)
			if err != nil {
				log.With(payload).Errorf("create completion failed: %v", err)
				panic(err)
			}
		}

		if prompt != payload.Prompt || negativePrompt != payload.NegativePrompt {
			argUpdate := repo2.CreativeRecordUpdateExtArgs{}
			if prompt != payload.Prompt {
				argUpdate.RealPrompt = prompt
			}

			if negativePrompt != payload.NegativePrompt {
				argUpdate.RealNegativePrompt = negativePrompt
			}

			if err := rep.Creative.UpdateRecordArgumentsByTaskID(ctx, payload.GetUID(), payload.GetID(), argUpdate); err != nil {
				log.WithFields(log.Fields{"payload": payload}).Errorf("update creative arguments failed: %s", err)
			}
		}

		if err := rep.Queue.CreatePendingTask(ctx, &repo2.PendingTask{
			TaskID:        payload.GetID(),
			TaskType:      TypeDashscopeImageCompletion,
			NextExecuteAt: time.Now().Add(time.Duration(5) * time.Second),
			DeadlineAt:    time.Now().Add(30 * time.Minute),
			Status:        repo2.PendingTaskStatusProcessing,
			Payload:       DashscopeImagePendingTaskPayload{DashscopeImageTaskID: resp.Output.TaskID, Payload: payload},
		}); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("create pending task failed: %s", err)
			panic(err)
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo2.QueueTaskStatusRunning,
			nil,
		)
	}
}

func dashscopeImageAsyncJobProcesser(que *Queue, client *dashscope2.DashScope, up *uploader.Uploader, rep *repo2.Repository) PendingTaskHandler {
	return func(task *model.QueueTasksPending) (update *repo2.PendingTaskUpdate, err error) {
		var payload DashscopeImagePendingTaskPayload
		if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
			return nil, err
		}

		taskRes, err := client.ImageTaskStatus(context.TODO(), payload.DashscopeImageTaskID)
		if err != nil {
			log.With(payload).Errorf("query fromston job result failed: %v", err)
			return &repo2.PendingTaskUpdate{
				NextExecuteAt: time.Now().Add(5 * time.Second),
				Status:        repo2.PendingTaskStatusProcessing,
				ExecuteTimes:  task.ExecuteTimes + 1,
			}, nil
		}

		defer func() {
			if err2 := recover(); err2 != nil {
				log.With(task).Errorf("panic: %v", err2)
				err = err2.(error)

				// 更新创作岛历史记录
				if err := rep.Creative.UpdateRecordByTaskID(context.TODO(), payload.Payload.GetUID(), payload.Payload.GetID(), repo2.CreativeRecordUpdateRequest{
					Answer: err.Error(),
					Status: repo2.CreativeStatusFailed,
				}); err != nil {
					log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
				}

				update = &repo2.PendingTaskUpdate{Status: repo2.PendingTaskStatusFailed}
			}

			if err != nil {
				if err := rep.Queue.Update(
					context.TODO(),
					payload.Payload.GetID(),
					repo2.QueueTaskStatusFailed,
					ErrorResult{
						Errors: []string{err.Error()},
					},
				); err != nil {
					log.With(task).Errorf("update queue status failed: %s", err)
				}
			}
		}()

		if taskRes.Output.TaskStatus == dashscope2.TaskStatusPending || taskRes.Output.TaskStatus == dashscope2.TaskStatusRunning {
			return &repo2.PendingTaskUpdate{
				NextExecuteAt: time.Now().Add(5 * time.Second),
				Status:        repo2.PendingTaskStatusProcessing,
				ExecuteTimes:  task.ExecuteTimes + 1,
			}, nil
		}

		// 任务已经完成，开始处理结果
		if taskRes.Output.TaskStatus == dashscope2.TaskStatusFailed {
			log.WithFields(log.Fields{"payload": payload, "task": task}).Errorf("no success task found")
			panic(errors.New("task failed: " + taskRes.Output.TaskStatus))
		}

		// 更新创作岛历史记录
		if err := handleDashscopeImageTask(que, payload, taskRes, up, rep); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			return nil, err
		}

		return &repo2.PendingTaskUpdate{Status: repo2.PendingTaskStatusSuccess}, nil
	}
}

type DashscopeImageTaskPayload interface {
	GetID() string
	GetUID() int64
	GetQuota() int64
	GetModel() string
	GetImage() string
}

func handleDashscopeImageTask(
	que *Queue,
	payload DashscopeImageTaskPayload,
	tasks *dashscope2.ImageTaskResponse,
	up *uploader.Uploader,
	rep *repo2.Repository,
) error {
	resources := array.Map(tasks.Output.Results, func(item dashscope2.ImageTaskOutputImage, _ int) string {
		return item.URL
	})
	resources = array.Filter(resources, func(item string, _ int) bool { return item != "" })

	if len(resources) == 0 {
		log.WithFields(log.Fields{
			"payload": payload,
		}).Errorf("没有生成任何图片")
		panic(errors.New("没有生成任何图片"))
	}

	// 更新创作岛历史记录状态，写入生成的图片资源地址
	retJson, err := json.Marshal(resources)
	if err != nil {
		log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
		panic(err)
	}

	// 重新计算配额消耗，以实际发生计算
	// quotaConsumed := coins.GetDashscopeImageImageCoins(payload.GetModel(), isCsMode, width, height) * int64(len(resources))
	quotaConsumed := int64(coins.GetUnifiedImageGenCoins("") * len(resources))

	req := repo2.CreativeRecordUpdateRequest{
		Answer:    string(retJson),
		QuotaUsed: quotaConsumed,
		Status:    repo2.CreativeStatusSuccess,
	}
	if err := rep.Creative.UpdateRecordByTaskID(context.TODO(), payload.GetUID(), payload.GetID(), req); err != nil {
		log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
		panic(err)
	}

	// 更新用户配额
	modelUsed := []string{payload.GetModel(), "upload"}
	if err := rep.Quota.QuotaConsume(
		context.TODO(),
		payload.GetUID(),
		payload.GetQuota(),
		repo2.NewQuotaUsedMeta("fromston", modelUsed...),
	); err != nil {
		log.Errorf("used quota add failed: %s", err)
		return err
	}

	// 触发文件下载上传七牛云任务
	downloadPayload := ImageDownloaderPayload{
		CreativeHistoryTaskID: payload.GetID(),
		UserID:                payload.GetUID(),
		CreatedAt:             time.Now(),
	}
	downloadTaskID, err := que.Enqueue(&downloadPayload, NewImageDownloaderTask)
	if err != nil {
		log.WithFields(log.Fields{"payload": payload}).Errorf("enqueue image downloader task failed: %s", err)
	} else {
		log.WithFields(log.Fields{"payload": payload, "task_id": downloadTaskID}).Debugf("enqueue image downloader task success")
	}

	// 更新队列任务状态
	return rep.Queue.Update(
		context.TODO(),
		payload.GetID(),
		repo2.QueueTaskStatusSuccess,
		CompletionResult{
			OriginImage: payload.GetImage(),
			Resources:   resources,
			ValidBefore: time.Now().Add(7 * 24 * time.Hour),
		},
	)
}
