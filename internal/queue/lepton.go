package queue

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/pkg/ai/lepton"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/image"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"github.com/mylxsw/asteria/log"
	"time"
)

type ArtisticTextCompletionPayload struct {
	ID             string    `json:"id,omitempty"`
	ArtisticType   string    `json:"artistic_type,omitempty"`
	Quota          int64     `json:"quota,omitempty"`
	UID            int64     `json:"uid,omitempty"`
	Prompt         string    `json:"prompt,omitempty"`
	NegativePrompt string    `json:"negative_prompt,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`

	Type     string `json:"type,omitempty"`
	Text     string `json:"text,omitempty"`
	FontPath string `json:"font_path,omitempty"`

	AIRewrite    bool  `json:"ai_rewrite,omitempty"`
	FreezedCoins int64 `json:"freezed_coins,omitempty"`

	ControlImageRatio float64 `json:"control_image_ratio,omitempty"`
	ControlWeight     float64 `json:"control_weight,omitempty"`
	GuidanceStart     float64 `json:"guidance_start,omitempty"`
	GuidanceEnd       float64 `json:"guidance_end,omitempty"`
	Seed              int64   `json:"seed,omitempty"`
	Steps             int64   `json:"steps,omitempty"`
	CfgScale          int64   `json:"cfg_scale,omitempty"`
	NumImages         int64   `json:"num_images,omitempty"`
}

func (payload *ArtisticTextCompletionPayload) GetTitle() string {
	return payload.Prompt
}

func (payload *ArtisticTextCompletionPayload) GetID() string {
	return payload.ID
}

func (payload *ArtisticTextCompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *ArtisticTextCompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *ArtisticTextCompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func NewArtisticTextCompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeArtisticTextCompletion, data)
}

func BuildArtisticTextCompletionHandler(client *lepton.Lepton, translator youdao.Translater, up *uploader.Uploader, rep *repo2.Repository, oai openai.Client) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload ArtisticTextCompletionPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		if err := rep.Queue.Update(context.TODO(), payload.GetID(), repo2.QueueTaskStatusRunning, nil); err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("set task status to running failed: %s", err)
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

		var prompt, negativePrompt string
		prompt, negativePrompt, payload.AIRewrite = resolvePrompts(
			ctx,
			PromptResolverPayload{
				Prompt:         payload.Prompt,
				NegativePrompt: payload.NegativePrompt,
				AIRewrite:      payload.AIRewrite,
			},
			rep.Creative,
			oai, translator,
		)

		imager := image.New(payload.FontPath)

		var maskData []byte
		switch payload.Type {
		case "text":
			maskData, err = imager.TextImage(payload.Text, 768)
			if err != nil {
				log.With(payload).Errorf("create text image failed: %v", err)
				panic(fmt.Errorf("艺术字生成失败: %w", err))
			}
		case "qr":
			maskData, err = imager.QR(payload.Text, 768)
			if err != nil {
				log.With(payload).Errorf("create qr image failed: %v", err)
				panic(fmt.Errorf("艺术二维码生成失败: %w", err))
			}
		default:
			panic(fmt.Errorf("不支持该艺术字类型: %v", payload.Type))
		}

		res, err := client.ImageGenerate(ctx, lepton.QRImageRequest{
			Prompt:            prompt,
			NegativePrompt:    negativePrompt,
			Model:             payload.ArtisticType,
			ControlImage:      base64.StdEncoding.EncodeToString(maskData),
			ControlImageRatio: payload.ControlImageRatio,
			ControlWeight:     payload.ControlWeight,
			GuidanceStart:     payload.GuidanceStart,
			GuidanceEnd:       payload.GuidanceEnd,
			Seed:              payload.Seed,
			Steps:             payload.Steps,
			CfgScale:          payload.CfgScale,
			NumImages:         payload.NumImages,
		})
		if err != nil {
			log.With(payload).Errorf("create completion failed: %v", err)
			// 此处故意 panic， 进入 defer 逻辑，以便更新 creative 状态
			panic(err)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		imageUrls, err := res.UploadResources(ctx, up, payload.GetUID())
		if err != nil {
			log.With(payload).Errorf("上传图片到七牛云存储失败: %", err)
			panic(fmt.Errorf("上传图片到七牛云存储失败: %w", err))
		}

		modelUsed := []string{"leptonai", "upload"}
		if len(imageUrls) == 0 {
			log.WithFields(log.Fields{
				"payload": payload,
			}).Errorf("没有生成任何图片")
			panic(errors.New("没有生成任何图片"))
		}

		// 更新创作岛历史记录
		retJson, err := json.Marshal(imageUrls)
		if err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			panic(err)
		}

		req := repo2.CreativeRecordUpdateRequest{
			Status:    repo2.CreativeStatusSuccess,
			Answer:    string(retJson),
			QuotaUsed: payload.GetQuota(),
		}

		if prompt != payload.Prompt || negativePrompt != payload.NegativePrompt {
			ext := repo2.CreativeRecordUpdateExtArgs{}
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
			repo2.NewQuotaUsedMeta("leptonai", modelUsed...),
		); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo2.QueueTaskStatusSuccess,
			CompletionResult{
				Resources:   imageUrls,
				ValidBefore: time.Now().Add(7 * 24 * time.Hour),
			},
		)
	}
}
