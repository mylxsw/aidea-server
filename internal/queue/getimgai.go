package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/getimgai"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/misc"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	uploader2 "github.com/mylxsw/aidea-server/pkg/uploader"
	youdao2 "github.com/mylxsw/aidea-server/pkg/youdao"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/mylxsw/go-utils/array"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/asteria/log"
)

type GetimgAICompletionPayload struct {
	ID    string `json:"id,omitempty"`
	Model string `json:"model,omitempty"`
	Quota int64  `json:"quota,omitempty"`
	UID   int64  `json:"uid,omitempty"`

	Prompt         string   `json:"prompt,omitempty"`
	PromptTags     []string `json:"prompt_tags,omitempty"`
	NegativePrompt string   `json:"negative_prompt,omitempty"`
	Width          int64    `json:"width,omitempty"`
	Height         int64    `json:"height,omitempty"`
	Steps          int64    `json:"steps,omitempty"`
	Image          string   `json:"image,omitempty"`
	AIRewrite      bool     `json:"ai_rewrite,omitempty"`
	Seed           int64    `json:"seed,omitempty"`
	ImageStrength  float64  `json:"image_strength,omitempty"`
	UpscaleBy      string   `json:"upscale_by,omitempty"`
	FilterID       int64    `json:"filter_id,omitempty"`

	CreatedAt    time.Time `json:"created_at,omitempty"`
	FreezedCoins int64     `json:"freezed_coins,omitempty"`
}

func (payload *GetimgAICompletionPayload) GetTitle() string {
	return payload.Prompt
}

func (payload *GetimgAICompletionPayload) GetID() string {
	return payload.ID
}

func (payload *GetimgAICompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *GetimgAICompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *GetimgAICompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func NewGetimgAICompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeGetimgAICompletion, data)
}

func BuildGetimgAICompletionHandler(client *getimgai.GetimgAI, translator youdao2.Translater, up *uploader2.Uploader, rep *repo2.Repository, oai openai.Client) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload GetimgAICompletionPayload
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
		var localImageBase64 string
		if payload.Image != "" {
			imagePath, err := uploader2.DownloadRemoteFile(ctx, payload.Image)
			if err != nil {
				log.WithFields(log.Fields{
					"payload": payload,
				}).Errorf("下载远程图片失败: %s", err)
				panic(err)
			}

			localImageBase64, err = misc.ImageToRawBase64(imagePath)
			if err != nil {
				log.WithFields(log.Fields{
					"payload": payload,
				}).Errorf("图片转换失败: %s", err)
				panic(err)
			}

			defer os.Remove(imagePath)
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
				Vendor:         "getimgai",
				Model:          payload.Model,
			},
			rep.Creative,
			oai, translator,
		)

		var resp *getimgai.ImageResponse
		if localImageBase64 != "" {
			resp, err = client.ImageToImage(ctx, getimgai.ImageToImageRequest{
				Model:          payload.Model,
				Prompt:         prompt,
				NegativePrompt: negativePrompt,
				Image:          localImageBase64,
				Steps:          payload.Steps,
				Strength:       payload.ImageStrength,
				OutputFormat:   "png",
			})
		} else {
			resp, err = client.TextToImage(ctx, getimgai.TextToImageRequest{
				Model:          payload.Model,
				Prompt:         prompt,
				NegativePrompt: negativePrompt,
				Width:          payload.Width,
				Height:         payload.Height,
				Seed:           payload.Seed,
				Steps:          payload.Steps,
				OutputFormat:   "png",
			})
		}

		if err != nil {
			log.With(payload).Errorf("create completion failed: %v", err)
			panic(err)
		}

		// 发起图片放大请求
		// getimg.ai 只支持放大 4 倍
		if payload.UpscaleBy != "" && payload.UpscaleBy != "x1" {
			upscaleRes, err := client.Upscale(ctx, getimgai.UpscaleRequest{
				Model:        "real-esrgan-4x",
				Image:        resp.Image,
				Scale:        4,
				OutputFormat: "png",
			})
			if err != nil {
				log.With(payload).Errorf("upscale failed: %v", err)
			} else {
				resp.Image = upscaleRes.Image
			}
		}

		resources, err := resp.UploadResources(ctx, up, payload.GetUID())
		if err != nil {
			log.WithFields(log.Fields{
				"payload": payload,
			}).Errorf(err.Error())
			panic(err)
		}

		if resources == "" {
			log.WithFields(log.Fields{
				"payload": payload,
			}).Errorf("没有生成任何图片")
			panic(errors.New("没有生成任何图片"))
		}

		modelUsed := []string{payload.Model, "upload"}

		// 更新创作岛历史记录

		retJson, err := json.Marshal([]string{resources})
		if err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("update creative failed: %s", err)
			panic(err)
		}

		updateReq := repo2.CreativeRecordUpdateRequest{
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
			repo2.NewQuotaUsedMeta("getimageai", modelUsed...),
		); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}

		return rep.Queue.Update(
			context.TODO(),
			payload.GetID(),
			repo2.QueueTaskStatusSuccess,
			CompletionResult{
				Resources:   []string{resources},
				OriginImage: payload.Image,
				ValidBefore: time.Now().Add(7 * 24 * time.Hour),
			},
		)
	}
}

type PromptResolverPayload struct {
	Prompt         string
	PromptTags     []string
	NegativePrompt string
	FilterID       int64
	AIRewrite      bool
	Image          string
	Vendor         string
	Model          string
}

func resolvePrompts(ctx context.Context, payload PromptResolverPayload, creativeRepo *repo2.CreativeRepo, oai openai.Client, translator youdao2.Translater) (string, string, bool) {
	prompt := payload.Prompt
	negativePrompt := payload.NegativePrompt

	// 查询 Filter 信息
	if payload.FilterID > 0 {
		filter, err := creativeRepo.Filter(ctx, payload.FilterID)
		if err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("invalid filter: %s", err)
		}

		if filter != nil {
			if filter.ImageMeta.ShouldUseTemplate(prompt) {
				prompt = filter.ImageMeta.ApplyTemplate(prompt)
			}
		}

		if negativePrompt == "" {
			negativePrompt = filter.ImageMeta.NegativePrompt
		}

		// 图生图模式下，自动追加 filter 的 prompt
		if payload.Image != "" {
			if prompt == "" {
				prompt = filter.ImageMeta.Prompt
			} else {
				prompt = fmt.Sprintf("%s, %s", prompt, filter.ImageMeta.Prompt)
			}
		}
	}

	if prompt == "" {
		// 如果没有输入 prompt，则使用默认的 prompt
		// 注意，要停用 AI 自动改写，避免改写出奇怪的东西
		prompt = "a character from Anime style"
		payload.AIRewrite = false
	}

	if payload.AIRewrite && oai != nil {
		p2, np2 := AIRewriteSDPromptEnhanced(ctx, oai, prompt)
		if p2 != prompt {
			prompt = p2
			if negativePrompt == "" && np2 != "" {
				negativePrompt = np2
			}
		}
	}

	if translator != nil {
		if misc.IsChinese(prompt) {
			translateRes, err := translator.Translate(ctx, youdao2.LanguageAuto, youdao2.LanguageEnglish, prompt)
			if err != nil {
				log.With(payload).Errorf("translate failed: %v", err)
			} else {
				prompt = translateRes.Result
			}
		}

		if strings.TrimSpace(negativePrompt) != "" && misc.IsChinese(negativePrompt) {
			translateRes, err := translator.Translate(ctx, youdao2.LanguageAuto, youdao2.LanguageEnglish, negativePrompt)
			if err != nil {
				log.With(payload).Errorf("translate failed: %v", err)
			} else {
				negativePrompt = translateRes.Result
			}
		}
	}

	if strings.TrimSpace(prompt) == "" {
		prompt = "best quality, animation effect"
	}

	if strings.TrimSpace(negativePrompt) == "" {
		negativePrompt = "NSFW, out of frame, lowres, text, error, cropped, worst quality, low quality, jpeg artifacts, ugly, duplicate, morbid, mutilated, out of frame, extra fingers, mutated hands, poorly drawn hands, poorly drawn face, mutation, deformed, blurry, dehydrated, bad anatomy, bad proportions, extra limbs, cloned face, disfigured, gross proportions, malformed limbs, missing arms, missing legs, extra arms, extra legs, fused fingers, too many fingers, long neck, username, watermark, signature"
	}

	// 查询模型信息
	if payload.Model != "" {
		modelInDB, err := creativeRepo.Model(ctx, payload.Vendor, payload.Model)
		if err != nil {
			log.WithFields(log.Fields{"payload": payload}).Errorf("invalid model: %s", payload.Model)
		} else {
			if modelInDB.ImageMeta.ArtistStyle != "" {
				artistStyles := array.Filter(
					strings.Split(strings.ReplaceAll(modelInDB.ImageMeta.ArtistStyle, "，", ","), ","),
					func(item string, _ int) bool { return strings.TrimSpace(item) != "" },
				)

				if len(artistStyles) > 0 {
					var hasStyle bool
					for _, style := range artistStyles {
						if strings.Contains(prompt, style) {
							hasStyle = true
							break
						}
					}

					if !hasStyle {
						prompt = fmt.Sprintf("%s,%s", artistStyles[rand.Intn(len(artistStyles))], prompt)
					}
				}
			}
		}
	}

	if len(payload.PromptTags) > 0 {
		prompt = strings.Trim(prompt, ",") + "," + strings.Join(payload.PromptTags, ",")
	}

	return prompt, negativePrompt, payload.AIRewrite
}
