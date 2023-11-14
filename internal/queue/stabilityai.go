package queue

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/ai/stabilityai"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	uploader2 "github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/asteria/log"
)

type StabilityAICompletionPayload struct {
	ID    string `json:"id,omitempty"`
	Model string `json:"model,omitempty"`
	Quota int64  `json:"quota,omitempty"`
	UID   int64  `json:"uid,omitempty"`

	Prompt         string   `json:"prompt,omitempty"`
	PromptTags     []string `json:"prompt_tags,omitempty"`
	NegativePrompt string   `json:"negative_prompt,omitempty"`
	ImageCount     int64    `json:"image_count,omitempty"`
	Width          int64    `json:"width,omitempty"`
	Height         int64    `json:"height,omitempty"`
	StylePreset    string   `json:"style_preset,omitempty"`
	Steps          int64    `json:"steps,omitempty"`
	Image          string   `json:"image,omitempty"`
	AIRewrite      bool     `json:"ai_rewrite,omitempty"`
	Seed           int64    `json:"seed,omitempty"`
	ImageStrength  float64  `json:"image_strength,omitempty"`
	FilterID       int64    `json:"filter_id,omitempty"`

	CreatedAt    time.Time `json:"created_at,omitempty"`
	FreezedCoins int64     `json:"freezed_coins,omitempty"`
}

func (payload *StabilityAICompletionPayload) GetTitle() string {
	return payload.Prompt
}

func (payload *StabilityAICompletionPayload) GetID() string {
	return payload.ID
}

func (payload *StabilityAICompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *StabilityAICompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *StabilityAICompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func NewStabilityAICompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeStabilityAICompletion, data)
}

func BuildStabilityAICompletionHandler(client *stabilityai.StabilityAI, translator youdao.Translater, up *uploader2.Uploader, rep *repo2.Repository, oai openai.Client) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload StabilityAICompletionPayload
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
		var localImagePath string
		if payload.Image != "" {
			imagePath, err := uploader2.DownloadRemoteFile(ctx, payload.Image)
			if err != nil {
				log.WithFields(log.Fields{
					"payload": payload,
				}).Errorf("下载远程图片失败: %s", err)
				panic(err)
			}

			localImagePath = imagePath
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
				Vendor:         "stabilityai",
				Model:          payload.Model,
			},
			rep.Creative,
			oai, translator,
		)

		var resp *stabilityai.TextToImageResponse
		if localImagePath != "" {
			resp, err = client.ImageToImage(ctx, payload.Model, stabilityai.ImageToImageRequest{
				TextPrompt:  prompt,
				InitImage:   localImagePath,
				CfgScale:    7,
				Samples:     1,
				Seed:        int(payload.Seed),
				Steps:       int(payload.Steps),
				StylePreset: payload.StylePreset,
				// Sampler:     "K_DPMPP_2M",
				ImageStrength: 1.0 - payload.ImageStrength,
			})
		} else {
			prompts := []stabilityai.TextPrompts{{Text: prompt, Weight: 0.9}}
			if negativePrompt != "" {
				prompts = append(prompts, stabilityai.TextPrompts{Text: negativePrompt, Weight: -0.5})
			}

			resp, err = client.TextToImage(payload.Model, stabilityai.TextToImageRequest{
				TextPrompts: prompts,
				Width:       int(payload.Width),
				Height:      int(payload.Height),
				CfgScale:    7,
				Samples:     int(payload.ImageCount),
				Seed:        int(payload.Seed),
				Steps:       int(payload.Steps),
				StylePreset: payload.StylePreset,
				// Sampler:     "K_DPMPP_2M",
			})
		}

		if err != nil {
			log.With(payload).Errorf("[StabilityAI] 图片生成失败: %v", err)
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
			repo2.NewQuotaUsedMeta("stabilityai", modelUsed...),
		); err != nil {
			log.Errorf("used quota add failed: %s", err)
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

var stableDiffusionPrompt = `You are an artsy Stable Diffusion prompt assistant, and based on the theme I provided, you need to generate a prompt describing the image. Prompt consists of a series of comma-separated words or phrases, called tags. You can use parentheses and square brackets to adjust the strength of keywords. Prompt should include the main body of the picture, materials, additional details, image quality, art style, color tone and lighting, but should not contain paragraph descriptions, colons or periods. For human subjects, the eyes, nose, and lips must be described to avoid distortion of facial features. The number of Tags cannot exceed 40, and they must be arranged in order of importance from high to low. The number of words is limited to 60, and the prompt you reply must be in English.`

func AIRewriteSDPrompt(ctx context.Context, oai openai.Client, userPrompt string) string {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	answer, err := oai.QuickAsk(ctx, stableDiffusionPrompt, userPrompt, 300)
	if err != nil {
		log.Errorf("ai rewrite failed: %s", err)
		return userPrompt
	}

	if answer != "" {
		log.WithFields(log.Fields{
			"prompt":  userPrompt,
			"rewrite": answer,
		}).Debugf("ai rewrite prompt success")
		return answer
	}

	return userPrompt
}

var stableDiffusionPromptEnhanced = `As an artistic assistant, your task is to create detailed prompts for Stable Diffusion to generate high-quality images based on themes I'll provide. 

## Prompt Concept
- Prompts comprise of a "Prompt:" and "Negative Prompt:" section, filled with tags separated by commas.
- Tags describe image content or elements to exclude in the generated image.

## () and [] Syntax
Brackets adjust keyword strength. (keyword) increases strength by 1.1 times while [keyword] reduces it by 0.9 times.

## Prompt Format Requirements
Prompts should detail people, scenery, objects, or abstract digital artworks and include at least five visual details.

### 1. Prompt Requirements
- Describe the main subject, texture, additional details, image quality, art style, color tone, and lighting. Avoid segmented descriptions, ":" or ".".
- For themes related to people, describe the eyes, nose, and lips to avoid deformation. Also detail appearance, emotions, clothing, posture, perspective, actions, background, etc.
- Texture refers to the artwork material.
- Image quality should start with "(best quality, 4k, 8k, highres, masterpiece:1.2), ultra-detailed, (realistic, photorealistic, photo-realistic:1.37),".
- Include the art style and control the image's overall color.
- Describe the image's lighting.

### 2. Negative Prompt Requirements
- Exclude: "nsfw, (low quality, normal quality, worst quality, jpeg artifacts), cropped, monochrome, lowres, low saturation, ((watermark)), (white letters)".
- For themes related to people, also exclude: "skin spots, acnes, skin blemishes, age spots, mutated hands, mutated fingers, deformed, bad anatomy, disfigured, poorly drawn face, extra limb, ugly, poorly drawn hands, missing limb, floating limbs, disconnected limbs, out of focus, long neck, long body, extra fingers, fewer fingers, (multi nipples), bad hands, signature, username, bad feet, blurry, bad body".

### 3. Limitations:
- Tags should be English words or phrases, not necessarily provided by me, with no sentences or explanations.
- Keep tag count within 40 and word count within 60.
- Exclude quotation marks("") in tags, and separate tags by commas.
- Arrange tags in order of importance.
- Themes may be in Chinese, but your output must be in English. 

Output as a json, with 'prompt' and 'negative_prompt' as keys.`

var defaultNegativePrompt = "out of frame, lowres, text, error, cropped, worst quality, low quality, jpeg artifacts, ugly, duplicate, morbid, mutilated, out of frame, extra fingers, mutated hands, poorly drawn hands, poorly drawn face, mutation, deformed, blurry, dehydrated, bad anatomy, bad proportions, extra limbs, cloned face, disfigured, gross proportions, malformed limbs, missing arms, missing legs, extra arms, extra legs, fused fingers, too many fingers, long neck, username, watermark, signature"

type PromptArg struct {
	Prompt          string `json:"prompt,omitempty"`
	NegativePrompt1 string `json:"negativePrompt,omitempty"`
	NegativePrompt2 string `json:"negative_prompt,omitempty"`
}

func AIRewriteSDPromptEnhanced(ctx context.Context, oai openai.Client, userPrompt string) (string, string) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	answer, err := oai.QuickAsk(ctx, stableDiffusionPromptEnhanced, userPrompt, 300)
	if err != nil {
		log.Errorf("ai rewrite failed: %s", err)
		return userPrompt, defaultNegativePrompt
	}

	if answer != "" {
		// 提取 prompt 和 negative prompt
		var arg PromptArg
		if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &arg); err != nil {
			return userPrompt, defaultNegativePrompt
		}

		arg.Prompt = regexp.MustCompile(`[\w\s]+:`).ReplaceAllString(arg.Prompt, "")
		arg.NegativePrompt1 = regexp.MustCompile(`[\w\s]+:`).ReplaceAllString(arg.NegativePrompt1, "")
		arg.NegativePrompt2 = regexp.MustCompile(`[\w\s]+:`).ReplaceAllString(arg.NegativePrompt2, "")

		negativePrompt := arg.NegativePrompt1
		if arg.NegativePrompt1 == "" {
			negativePrompt = arg.NegativePrompt2
		}

		log.WithFields(log.Fields{
			"prompt":  userPrompt,
			"rewrite": answer,
		}).Debugf("ai rewrite prompt success")
		return arg.Prompt, negativePrompt
	}

	return userPrompt, defaultNegativePrompt
}
