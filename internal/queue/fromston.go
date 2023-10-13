package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/ai/fromston"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/repo/model"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/ternary"
)

type FromStonCompletionPayload struct {
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

	Image         string  `json:"image,omitempty"`
	AIRewrite     bool    `json:"ai_rewrite,omitempty"`
	ImageStrength float64 `json:"image_strength,omitempty"`
	FilterID      int64   `json:"filter_id,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty"`
}

func (payload *FromStonCompletionPayload) GetTitle() string {
	return payload.Prompt
}

func (payload *FromStonCompletionPayload) GetID() string {
	return payload.ID
}

func (payload *FromStonCompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *FromStonCompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *FromStonCompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func (payload *FromStonCompletionPayload) GetModel() string {
	return payload.Model
}

func NewFromStonCompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeFromStonCompletion, data)
}

type FromStonPendingTaskPayload struct {
	FromstonTaskIDs []string                  `json:"fromston_task_ids,omitempty"`
	Payload         FromStonCompletionPayload `json:"payload,omitempty"`
	ModelType       string                    `json:"model_type,omitempty"`
}

func (p FromStonPendingTaskPayload) GetImage() string {
	return p.Payload.Image
}

func (p FromStonPendingTaskPayload) GetID() string {
	return p.Payload.GetID()
}

func (p FromStonPendingTaskPayload) GetUID() int64 {
	return p.Payload.UID
}

func (p FromStonPendingTaskPayload) GetQuota() int64 {
	return p.Payload.Quota
}

func (p FromStonPendingTaskPayload) GetModel() string {
	return p.Payload.Model
}

type FromStonResponse interface {
	GetID() string
	GetState() string
	IsFinished() bool
	IsProcessing() bool
	UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) ([]string, error)
	GetImages() []string
}

func BuildFromStonCompletionHandler(client *fromston.Fromston, up *uploader.Uploader, rep *repo.Repository) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload FromStonCompletionPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
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
					Answer: err.Error(),
					Status: repo.CreativeStatusFailed,
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

		// 下载远程图片（图生图）
		// 先尝试本地下载，成功则发送文件到 Leap
		// 如果本地下载失败，则直接发送远程图片地址到 Leap
		localImagePath := payload.Image
		if payload.Image != "" {
			imagePath, err := uploader.DownloadRemoteFile(ctx, payload.Image)
			if err != nil {
				log.WithFields(log.Fields{
					"payload": payload,
				}).Errorf("下载远程图片失败: %s", err)
			} else {
				localImagePath = imagePath
				defer os.Remove(imagePath)
			}

			uploadPath, err := client.UploadImage(ctx, localImagePath)
			if err != nil {
				log.WithFields(log.Fields{
					"payload": payload,
				}).Errorf("上传用户图片到 6Pen 失败: %s", err)
			} else {
				localImagePath = uploadPath
			}
		}

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
				Vendor:         "fromston",
				Model:          payload.Model,
			},
			rep.Creative,
			nil,
			nil,
		)

		prompt = helper.WordTruncate(prompt, 500)
		negativePrompt = helper.WordTruncate(negativePrompt, 500)

		ms := strings.SplitN(payload.GetModel(), ":", 2)
		if len(ms) != 2 {
			panic(fmt.Errorf("invalid model: %s", payload.Model))
		}

		modelType, modelIdStr := ms[0], ms[1]

		var resp *fromston.GenImageResponseData
		if modelType == "custom" {
			// 自己训练的模型
			req := fromston.GenImageCustomRequest{
				Prompt:     prompt,
				FillPrompt: int64(ternary.If(payload.AIRewrite, 1, 0)),
				Width:      payload.Width,
				Height:     payload.Height,
				RefImg:     localImagePath,
				ModelID:    modelIdStr,
				Multiply:   payload.ImageCount,
				Addition: &fromston.GenImageAddition{
					ImgFmt:         "jpg",
					NegativePrompt: negativePrompt,
					Strength:       payload.ImageStrength,
					CfgScale:       7,
				},
			}

			resp, err = client.GenImageCustom(ctx, req)
			if err != nil {
				log.With(payload).Errorf("create completion failed: %v", err)
				panic(err)
			}
		} else {
			modelId, err := strconv.Atoi(modelIdStr)
			if err != nil {
				panic(fmt.Errorf("invalid model: %s", payload.Model))
			}

			req := fromston.GenImageRequest{
				Prompt:     prompt,
				FillPrompt: int64(ternary.If(payload.AIRewrite, 1, 0)),
				Width:      payload.Width,
				Height:     payload.Height,
				RefImg:     localImagePath,
				ModelType:  ms[0],
				ModelID:    int64(modelId),
				Multiply:   payload.ImageCount,
				Addition: &fromston.GenImageAddition{
					ImgFmt:         "jpg",
					NegativePrompt: negativePrompt,
					Strength:       payload.ImageStrength,
					CfgScale:       7,
				},
			}

			resp, err = client.GenImage(ctx, req)
			if err != nil {
				log.With(payload).Errorf("create completion failed: %v", err)
				panic(err)
			}
		}

		estimates := array.Reduce(resp.Estimates, func(carry int64, item int64) int64 { return carry + item }, 0)
		log.WithFields(log.Fields{
			"estimates": estimates,
		}).Debugf("create fromston completion task finished: %v", resp.IDs)
		// 限制最大等待时间为 15 秒
		if estimates > 15 {
			estimates = 15
		}

		if prompt != payload.Prompt || negativePrompt != payload.NegativePrompt {
			argUpdate := repo.CreativeRecordUpdateExtArgs{}
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

		if err := rep.Queue.CreatePendingTask(ctx, &repo.PendingTask{
			TaskID:        payload.GetID(),
			TaskType:      TypeFromStonCompletion,
			NextExecuteAt: time.Now().Add(time.Duration(estimates) * time.Second),
			DeadlineAt:    time.Now().Add(30 * time.Minute),
			Status:        repo.PendingTaskStatusProcessing,
			Payload:       FromStonPendingTaskPayload{FromstonTaskIDs: resp.IDs, Payload: payload, ModelType: modelType},
		}); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("create pending task failed: %s", err)
			panic(err)
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo.QueueTaskStatusRunning,
			nil,
		)
	}
}

func fromStonAsyncJobProcesser(que *Queue, client *fromston.Fromston, up *uploader.Uploader, rep *repo.Repository) PendingTaskHandler {
	return func(task *model.QueueTasksPending) (update *repo.PendingTaskUpdate, err error) {
		var payload FromStonPendingTaskPayload
		if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
			return nil, err
		}

		var tasks []fromston.Task
		if payload.ModelType == "custom" {
			// 自己训练的模型任务查询
			for _, id := range payload.FromstonTaskIDs {
				customTask, err := client.QueryCustomTask(context.TODO(), id)
				if err != nil {
					log.WithFields(log.Fields{"task_id": id}).Errorf("query fromston custom job result failed: %v", err)
					continue
				}

				tasks = append(tasks, *customTask)
			}
		} else {
			tasks, err = client.QueryTasks(context.TODO(), payload.FromstonTaskIDs)
			if err != nil {
				log.With(payload).Errorf("query fromston job result failed: %v", err)
				return &repo.PendingTaskUpdate{
					NextExecuteAt: time.Now().Add(5 * time.Second),
					Status:        repo.PendingTaskStatusProcessing,
					ExecuteTimes:  task.ExecuteTimes + 1,
				}, nil
			}
		}

		defer func() {
			if err2 := recover(); err2 != nil {
				log.With(task).Errorf("panic: %v", err2)
				err = err2.(error)

				// 更新创作岛历史记录
				if err := rep.Creative.UpdateRecordByTaskID(context.TODO(), payload.Payload.GetUID(), payload.Payload.GetID(), repo.CreativeRecordUpdateRequest{
					Answer: err.Error(),
					Status: repo.CreativeStatusFailed,
				}); err != nil {
					log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
				}

				update = &repo.PendingTaskUpdate{Status: repo.PendingTaskStatusFailed}
			}

			if err != nil {
				if err := rep.Queue.Update(
					context.TODO(),
					payload.Payload.GetID(),
					repo.QueueTaskStatusFailed,
					ErrorResult{
						Errors: []string{err.Error()},
					},
				); err != nil {
					log.With(task).Errorf("update queue status failed: %s", err)
				}
			}
		}()

		unfinishedTask := array.Filter(tasks, func(item fromston.Task, _ int) bool {
			return array.In(item.State, []string{"in_wait", "in_create"})
		})

		if len(unfinishedTask) > 0 {
			return &repo.PendingTaskUpdate{
				NextExecuteAt: time.Now().Add(5 * time.Second),
				Status:        repo.PendingTaskStatusProcessing,
				ExecuteTimes:  task.ExecuteTimes + 1,
			}, nil
		}

		// 任务已经完成，开始处理结果
		successTasks := array.Filter(tasks, func(item fromston.Task, _ int) bool {
			return item.State == "success"
		})

		if len(successTasks) == 0 {
			log.WithFields(log.Fields{"payload": payload, "tasks": tasks}).Errorf("no success task found")
			failedTasks := array.Filter(tasks, func(item fromston.Task, _ int) bool { return item.State == "fail" })
			if len(failedTasks) > 0 {
				panic(errors.New(strings.Join(array.Map(failedTasks, func(t fromston.Task, _ int) string { return t.FailReson }), "; ")))
			} else {
				panic(errors.New("fromston tasks failed"))
			}
		}

		// 更新创作岛历史记录
		if err := handleFromstonTask(que, payload, successTasks, up, rep); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			return nil, err
		}

		return &repo.PendingTaskUpdate{Status: repo.PendingTaskStatusSuccess}, nil
	}
}

type FromstonTaskPayload interface {
	GetID() string
	GetUID() int64
	GetQuota() int64
	GetModel() string
	GetImage() string
}

func handleFromstonTask(
	que *Queue,
	payload FromstonTaskPayload,
	tasks []fromston.Task,
	up *uploader.Uploader,
	rep *repo.Repository,
) error {
	resources := array.Map(tasks, func(item fromston.Task, _ int) string {
		return item.GenImg
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
	// quotaConsumed := coins.GetFromstonImageCoins(payload.GetModel(), isCsMode, width, height) * int64(len(resources))
	quotaConsumed := int64(coins.GetUnifiedImageGenCoins() * len(resources))

	req := repo.CreativeRecordUpdateRequest{
		Answer:    string(retJson),
		QuotaUsed: quotaConsumed,
		Status:    repo.CreativeStatusSuccess,
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
		repo.NewQuotaUsedMeta("fromston", modelUsed...),
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
		repo.QueueTaskStatusSuccess,
		CompletionResult{
			OriginImage: payload.GetImage(),
			Resources:   resources,
			ValidBefore: time.Now().Add(7 * 24 * time.Hour),
		},
	)
}
