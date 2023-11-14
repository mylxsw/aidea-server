package queue

import (
	"context"
	"encoding/json"
	"fmt"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/asteria/log"
)

type ImageDownloaderPayload struct {
	ID                    string    `json:"id,omitempty"`
	CreativeHistoryTaskID string    `json:"creative_history_task_id,omitempty"`
	UserID                int64     `json:"user_id,omitempty"`
	CreatedAt             time.Time `json:"created_at,omitempty"`
}

func (payload *ImageDownloaderPayload) GetTitle() string {
	return "图片下载"
}

func (payload *ImageDownloaderPayload) SetID(id string) {
	payload.ID = id
}

func (payload *ImageDownloaderPayload) GetID() string {
	return payload.ID
}

func (payload *ImageDownloaderPayload) GetUID() int64 {
	return payload.UserID
}

func (payload *ImageDownloaderPayload) GetQuotaID() int64 {
	return 0
}

func (payload *ImageDownloaderPayload) GetQuota() int64 {
	return 0
}

func NewImageDownloaderTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeImageDownloader, data)
}

func BuildImageDownloaderHandler(up *uploader.Uploader, rep *repo2.Repository) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload ImageDownloaderPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		log.With(payload).Debugf("开始处理图片下载任务")

		// 如果任务是 30 分钟前创建的，不再处理
		if payload.CreatedAt.Add(30 * time.Minute).Before(time.Now()) {
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

		item, err := rep.Creative.FindHistoryRecordByTaskId(ctx, payload.UserID, payload.CreativeHistoryTaskID)
		if err != nil {
			panic(fmt.Errorf("创作岛历史记录查询失败: %w", err))
		}

		if item.Answer == "" {
			log.With(payload).Errorf("创作岛历史记录中没有图片: %s", payload.ID)
			panic(fmt.Errorf("创作岛历史记录中没有图片: %s", payload.ID))
		}

		var resources []string
		if err := json.Unmarshal([]byte(item.Answer), &resources); err != nil {
			panic(fmt.Errorf("创作岛历史记录中图片解析失败: %w", err))
		}

		if len(resources) == 0 {
			log.With(payload).Errorf("创作岛历史记录中没有图片: %s", payload.ID)
			panic(fmt.Errorf("创作岛历史记录中没有图片: %s", payload.ID))
		}

		for i, res := range resources {
			ret, err := up.UploadRemoteFile(ctx, res, int(payload.UserID), uploader.DefaultUploadExpireAfterDays, "png", false)
			if err != nil {
				log.WithFields(log.Fields{
					"payload": payload,
				}).Errorf("图片上传失败: %s", err)
			} else {
				resources[i] = ret
			}
		}

		answer, _ := json.Marshal(resources)
		if err := rep.Creative.UpdateRecordAnswerByTaskID(ctx, payload.UserID, payload.CreativeHistoryTaskID, string(answer)); err != nil {
			log.WithFields(log.Fields{
				"payload": payload,
			}).Errorf("创作岛历史记录更新失败: %s", err)
			panic(fmt.Errorf("创作岛历史记录更新失败: %w", err))
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo2.QueueTaskStatusSuccess,
			EmptyResult{},
		)
	}
}
