package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/mylxsw/aidea-server/internal/ai/deepai"
	"github.com/mylxsw/aidea-server/internal/ai/stabilityai"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"github.com/mylxsw/asteria/log"

	// image.DecodeConfig 需要引入相关的图像包
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

type ImageUpscalePayload struct {
	ID                string    `json:"id,omitempty"`
	CreativeHistoryID int64     `json:"creative_history_id,omitempty"`
	Image             string    `json:"image,omitempty"`
	UserID            int64     `json:"user_id,omitempty"`
	UpscaleBy         string    `json:"upscale_by,omitempty"`
	Quota             int64     `json:"quota,omitempty"`
	CreatedAt         time.Time `json:"created_at,omitempty"`
}

func (payload *ImageUpscalePayload) GetTitle() string {
	return "超分辨率"
}

func (payload *ImageUpscalePayload) SetID(id string) {
	payload.ID = id
}

func (payload *ImageUpscalePayload) GetID() string {
	return payload.ID
}

func (payload *ImageUpscalePayload) GetUID() int64 {
	return payload.UserID
}

func (payload *ImageUpscalePayload) GetQuotaID() int64 {
	return 0
}

func (payload *ImageUpscalePayload) GetQuota() int64 {
	return payload.Quota
}

func NewImageUpscaleTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeImageUpscale, data)
}

func BuildImageUpscaleHandler(deepClient *deepai.DeepAI, client *stabilityai.StabilityAI, up *uploader.Uploader, rep *repo.Repository) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload ImageUpscalePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		log.With(payload).Debugf("开始处理图片超分辨率任务")

		// 如果任务是 30 分钟前创建的，不再处理
		if payload.CreatedAt.Add(30 * time.Minute).Before(time.Now()) {
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

		var resources []string
		if payload.CreativeHistoryID > 0 {
			item, err := rep.Creative.FindHistoryRecord(ctx, payload.UserID, payload.CreativeHistoryID)
			if err != nil {
				panic(fmt.Errorf("创作岛历史记录查询失败: %w", err))
			}

			if item.Answer == "" {
				log.With(payload).Errorf("创作岛历史记录中没有图片: %s", payload.ID)
				panic(fmt.Errorf("创作岛历史记录中没有图片: %s", payload.ID))
			}

			if err := json.Unmarshal([]byte(item.Answer), &resources); err != nil {
				panic(fmt.Errorf("创作岛历史记录中图片解析失败: %w", err))
			}

			if len(resources) == 0 {
				log.With(payload).Errorf("创作岛历史记录中没有图片: %s", payload.ID)
				panic(fmt.Errorf("创作岛历史记录中没有图片: %s", payload.ID))
			}

			for i, res := range resources {
				ret, err := upscaleBy(ctx, deepClient, client, up, payload.UserID, payload.UpscaleBy, res)
				if err != nil {
					panic(fmt.Errorf("图片超分辨率失败: %w", err))
				}

				resources[i] = ret
			}

			answer, _ := json.Marshal(resources)
			if err := rep.Creative.UpdateRecordAnswerByID(ctx, payload.UserID, payload.CreativeHistoryID, string(answer)); err != nil {
				log.WithFields(log.Fields{
					"payload": payload,
				}).Errorf("创作岛历史记录更新失败: %s", err)
				panic(fmt.Errorf("创作岛历史记录更新失败: %w", err))
			}
		} else {
			res, err := upscaleBy(ctx, deepClient, client, up, payload.UserID, payload.UpscaleBy, payload.Image)
			if err != nil {
				panic(fmt.Errorf("图片超分辨率失败: %w", err))
			}

			resources = []string{res}
			retJson, _ := json.Marshal(resources)
			updateReq := repo.CreativeRecordUpdateRequest{
				Status:    repo.CreativeStatusSuccess,
				Answer:    string(retJson),
				QuotaUsed: payload.GetQuota(),
			}

			if err := rep.Creative.UpdateRecordByTaskID(ctx, payload.GetUID(), payload.GetID(), updateReq); err != nil {
				log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
				return err
			}
		}

		// 记录消耗
		if err := rep.Quota.QuotaConsume(ctx, payload.GetUID(), payload.GetQuota(), repo.NewQuotaUsedMeta("upscale", "esrgan-v1-x2plus")); err != nil {
			log.With(payload).Errorf("used quota add failed: %s", err)
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo.QueueTaskStatusSuccess,
			CompletionResult{
				Resources:   resources,
				OriginImage: payload.Image,
				ValidBefore: time.Now().Add(7 * 24 * time.Hour),
			},
		)
	}
}

func upscaleBy(ctx context.Context, deepClient *deepai.DeepAI, client *stabilityai.StabilityAI, up *uploader.Uploader, userID int64, upscaleBy string, imageURL string) (string, error) {
	return upscaleByDeepAI(ctx, deepClient, up, userID, imageURL)
	//return upscaleByStabilityAI(ctx, client, up, userID, upscaleBy, imageURL)
}

func upscaleByDeepAI(ctx context.Context, deepClient *deepai.DeepAI, up *uploader.Uploader, userID int64, imageURL string) (string, error) {
	res, err := deepClient.Upscale(ctx, imageURL)
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

func upscaleByStabilityAI(ctx context.Context, client *stabilityai.StabilityAI, up *uploader.Uploader, userID int64, upscaleBy string, imageURL string) (string, error) {
	localPath, err := uploader.DownloadRemoteFile(ctx, imageURL)
	if err != nil {
		return "", fmt.Errorf("图片下载失败: %w", err)
	}
	defer os.Remove(localPath)

	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("图片打开失败: %w", err)
	}
	defer f.Close()

	var width, height int

	img, _, err := image.DecodeConfig(f)
	if err != nil {
		log.WithFields(log.Fields{
			"user_id":    userID,
			"local_file": localPath,
		}).Errorf("图片解码失败: %s", err)

		switch upscaleBy {
		case "x2":
			width, height = 2048, 2048
		case "x3":
			width, height = 3072, 3072
		case "x4":
			width, height = 4096, 4096
		default:
		}
	} else {
		switch upscaleBy {
		case "x2":
			width, height = img.Width*2, img.Height*2
		case "x3":
			width, height = img.Width*3, img.Height*3
		case "x4":
			width, height = img.Width*4, img.Height*4
		default:
		}
	}

	if width*height > 4194304 {
		width, height = 2048, 2048
	}

	if width == 0 || height == 0 {
		return "", fmt.Errorf("图片超分辨率失败，不支持的尺寸: %w", err)
	}

	res, err := client.Upscale(ctx, "esrgan-v1-x2plus", localPath, int64(width), int64(height))
	if err != nil {
		return "", fmt.Errorf("图片超分辨率失败: %w", err)
	}
	uploaded, err := res.UploadResources(ctx, up, userID)
	if err != nil {
		return "", fmt.Errorf("图片上传失败: %w", err)
	}

	if len(uploaded) == 0 {
		return "", fmt.Errorf("图片上传失败: %w", err)
	}

	return uploaded[0], nil
}
