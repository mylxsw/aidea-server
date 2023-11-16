package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/deepai"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"path/filepath"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/asteria/log"

	// image.DecodeConfig 需要引入相关的图像包
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

type ImageColorizationPayload struct {
	ID           string    `json:"id,omitempty"`
	Image        string    `json:"image,omitempty"`
	UserID       int64     `json:"user_id,omitempty"`
	Quota        int64     `json:"quota,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	FreezedCoins int64     `json:"freezed_coins,omitempty"`
}

func (payload *ImageColorizationPayload) GetTitle() string {
	return "图片上色"
}

func (payload *ImageColorizationPayload) SetID(id string) {
	payload.ID = id
}

func (payload *ImageColorizationPayload) GetID() string {
	return payload.ID
}

func (payload *ImageColorizationPayload) GetUID() int64 {
	return payload.UserID
}

func (payload *ImageColorizationPayload) GetQuotaID() int64 {
	return 0
}

func (payload *ImageColorizationPayload) GetQuota() int64 {
	return payload.Quota
}

func NewImageColorizationTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeImageColorization, data)
}

func BuildImageColorizationHandler(deepClient *deepai.DeepAI, up *uploader.Uploader, rep *repo2.Repository) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload ImageColorizationPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		log.With(payload).Debugf("开始处理图片上色任务")

		// 如果任务是 30 分钟前创建的，不再处理
		if payload.CreatedAt.Add(30 * time.Minute).Before(time.Now()) {
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

		var resources []string
		res, err := imageColorization(ctx, deepClient, up, payload.UserID, payload.Image)
		if err != nil {
			panic(fmt.Errorf("图片上色失败: %w", err))
		}

		resources = []string{res}
		retJson, _ := json.Marshal(resources)
		updateReq := repo2.CreativeRecordUpdateRequest{
			Status:    repo2.CreativeStatusSuccess,
			Answer:    string(retJson),
			QuotaUsed: payload.GetQuota(),
		}

		if err := rep.Creative.UpdateRecordByTaskID(ctx, payload.GetUID(), payload.GetID(), updateReq); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			return err
		}

		// 记录消耗
		if err := rep.Quota.QuotaConsume(ctx, payload.GetUID(), payload.GetQuota(), repo2.NewQuotaUsedMeta("upscale", "esrgan-v1-x2plus")); err != nil {
			log.With(payload).Errorf("used quota add failed: %s", err)
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo2.QueueTaskStatusSuccess,
			CompletionResult{
				Resources:   resources,
				OriginImage: payload.Image,
				ValidBefore: time.Now().Add(7 * 24 * time.Hour),
			},
		)
	}
}

func imageColorization(ctx context.Context, deepClient *deepai.DeepAI, up *uploader.Uploader, userID int64, imageURL string) (string, error) {
	res, err := deepClient.DrawColor(ctx, imageURL)
	if err != nil {
		return "", fmt.Errorf("图片超分辨率失败: %w", err)
	}

	uploaded, err := up.UploadRemoteFile(ctx, res.OutputURL, int(userID), uploader.DefaultUploadExpireAfterDays, filepath.Ext(res.OutputURL), false)
	if err != nil {
		return "", fmt.Errorf("图片上传失败: %w", err)
	}

	if uploaded == "" {
		return "", fmt.Errorf("图片上传失败: %w", err)
	}

	return uploaded, nil
}
