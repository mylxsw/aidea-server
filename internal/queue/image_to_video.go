package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/pkg/ai/stabilityai"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/asteria/log"
	"os"
	"time"
)

type ImageToVideoCompletionPayload struct {
	ID    string `json:"id,omitempty"`
	Quota int64  `json:"quota,omitempty"`
	UID   int64  `json:"uid,omitempty"`

	Seed  int64  `json:"seed,omitempty"`
	Image string `json:"image,omitempty"`

	Width  int64 `json:"width,omitempty"`
	Height int64 `json:"height,omitempty"`

	// CfgScale How strongly the video sticks to the original image.
	// Use lower values to allow the model more freedom to make changes and higher values to correct motion distortions.
	// number [ 0 .. 10 ], default 2.5
	CfgScale float64 `json:"cfg_scale,omitempty"`
	// MotionBucketID Lower values generally result in less motion in the output video,
	// while higher values generally result in more motion.
	// This parameter corresponds to the motion_bucket_id parameter from the paper.
	// number [ 1 .. 255 ], default 40
	MotionBucketID int `json:"motion_bucket_id,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty"`
}

func (payload *ImageToVideoCompletionPayload) GetTitle() string {
	return "图片转视频"
}

func (payload *ImageToVideoCompletionPayload) GetID() string {
	return payload.ID
}

func (payload *ImageToVideoCompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *ImageToVideoCompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *ImageToVideoCompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func (payload *ImageToVideoCompletionPayload) GetModel() string {
	return "stability-image-to-video"
}

func NewImageToVideoCompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeImageToVideoCompletion, data)
}

type ImageToVideoPendingTaskPayload struct {
	TaskID  string                        `json:"task_id,omitempty"`
	Payload ImageToVideoCompletionPayload `json:"payload,omitempty"`
}

func (p ImageToVideoPendingTaskPayload) GetImage() string {
	return p.Payload.Image
}

func (p ImageToVideoPendingTaskPayload) GetID() string {
	return p.Payload.GetID()
}

func (p ImageToVideoPendingTaskPayload) GetUID() int64 {
	return p.Payload.UID
}

func (p ImageToVideoPendingTaskPayload) GetQuota() int64 {
	return p.Payload.Quota
}

func (p ImageToVideoPendingTaskPayload) GetModel() string {
	return p.Payload.GetModel()
}

func BuildImageToVideoCompletionHandler(
	client *stabilityai.StabilityAI,
	rep *repo.Repository,
) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload ImageToVideoCompletionPayload
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

		targetImage, err := uploader.DownloadRemoteFile(ctx, payload.Image)
		if err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("download remote file failed: %s", err)
			panic(err)
		}
		defer os.Remove(targetImage)

		req := stabilityai.VideoRequest{
			ImagePath:      targetImage,
			Seed:           int(payload.Seed),
			CfgScale:       payload.CfgScale,
			MotionBucketID: payload.MotionBucketID,
		}
		resp, err := client.ImageToVideo(ctx, req)
		if err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("create task failed: %s", err)
			panic(err)
		}

		if err := rep.Queue.CreatePendingTask(ctx, &repo.PendingTask{
			TaskID:        payload.GetID(),
			TaskType:      TypeImageToVideoCompletion,
			NextExecuteAt: time.Now().Add(time.Duration(30) * time.Second),
			DeadlineAt:    time.Now().Add(30 * time.Minute),
			Status:        repo.PendingTaskStatusProcessing,
			Payload:       ImageToVideoPendingTaskPayload{TaskID: resp.ID, Payload: payload},
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

func imageToVideoJobProcesser(client *stabilityai.StabilityAI, up *uploader.Uploader, rep *repo.Repository) PendingTaskHandler {
	return func(task *model.QueueTasksPending) (update *repo.PendingTaskUpdate, err error) {
		var payload ImageToVideoPendingTaskPayload
		if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
			return nil, err
		}

		taskRes, err := client.ImageToVideoResult(context.TODO(), payload.TaskID)
		if err != nil {
			log.With(payload).Errorf("query image-to-video job result failed: %v", err)
			return &repo.PendingTaskUpdate{
				NextExecuteAt: time.Now().Add(5 * time.Second),
				Status:        repo.PendingTaskStatusProcessing,
				ExecuteTimes:  task.ExecuteTimes + 1,
			}, nil
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

		if taskRes.Video == "" {
			if taskRes.Status == "in-progress" {
				return &repo.PendingTaskUpdate{
					NextExecuteAt: time.Now().Add(5 * time.Second),
					Status:        repo.PendingTaskStatusProcessing,
					ExecuteTimes:  task.ExecuteTimes + 1,
				}, nil
			}

			log.WithFields(log.Fields{"payload": payload, "res": taskRes}).Errorf("no success task found")
			panic(errors.New("no success task found"))
		}

		// 任务已经完成，开始处理结果
		// 更新创作岛历史记录
		if err := handleImageToVideoTask(&payload, taskRes, up, rep); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			return nil, err
		}

		return &repo.PendingTaskUpdate{Status: repo.PendingTaskStatusSuccess}, nil
	}
}

func handleImageToVideoTask(
	payload *ImageToVideoPendingTaskPayload,
	tasks *stabilityai.VideoResponse,
	up *uploader.Uploader,
	rep *repo.Repository,
) error {
	videoURL, err := tasks.UploadResources(context.TODO(), up, payload.GetUID())
	if err != nil {
		return fmt.Errorf("upload resources failed: %s", err)
	}

	resources := make([]string, 0)
	resources = append(resources, videoURL)

	if len(resources) == 0 {
		log.WithFields(log.Fields{
			"payload": payload,
		}).Errorf("没有生成任何视频")
		panic(errors.New("没有生成任何视频"))
	}

	// 更新创作岛历史记录状态，写入生成的资源地址
	retJson, err := json.Marshal(resources)
	if err != nil {
		log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
		panic(err)
	}

	// 重新计算配额消耗，以实际发生计算
	quotaConsumed := int64(coins.GetUnifiedVideoGenCoins(payload.GetModel()) * len(resources))

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
		repo.NewQuotaUsedMeta(payload.GetModel(), modelUsed...),
	); err != nil {
		log.Errorf("used quota add failed: %s", err)
		return err
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
			Width:       payload.Payload.Width,
			Height:      payload.Payload.Height,
		},
	)
}
