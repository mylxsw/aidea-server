package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/aidea-server/internal/ai/deepai"
	"github.com/mylxsw/aidea-server/internal/ai/fromston"
	"github.com/mylxsw/aidea-server/internal/ai/getimgai"
	"github.com/mylxsw/aidea-server/internal/ai/leap"
	"github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/aidea-server/internal/ai/stabilityai"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
)

type ImageCompletionPayload struct {
	ID string `json:"id,omitempty"`

	Quota int64 `json:"quota,omitempty"`
	UID   int64 `json:"uid,omitempty"`

	Prompt         string    `json:"prompt,omitempty"`
	NegativePrompt string    `json:"negative_prompt,omitempty"`
	PromptTags     []string  `json:"prompt_tags,omitempty"`
	ImageCount     int64     `json:"image_count,omitempty"`
	ImageRatio     string    `json:"image_ratio,omitempty"`
	Width          int64     `json:"width,omitempty"`
	Height         int64     `json:"height,omitempty"`
	Steps          int64     `json:"steps,omitempty"`
	Image          string    `json:"image,omitempty"`
	AIRewrite      bool      `json:"ai_rewrite,omitempty"`
	Mode           string    `json:"mode,omitempty"`
	UpscaleBy      string    `json:"upscale_by,omitempty"`
	StylePreset    string    `json:"style_preset,omitempty"`
	Vendor         string    `json:"vendor,omitempty"`
	Model          string    `json:"model,omitempty"`
	Seed           int64     `json:"seed,omitempty"`
	ImageStrength  float64   `json:"image_strength,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	ModelName      string    `json:"model_name,omitempty"`
	FilterID       int64     `json:"filter_id,omitempty"`
	FilterName     string    `json:"filter_name,omitempty"`
	GalleryCopyID  int64     `json:"gallery_copy_id,omitempty"`
}

func (payload *ImageCompletionPayload) GetTitle() string {
	return payload.Prompt
}

func (payload *ImageCompletionPayload) GetID() string {
	return payload.ID
}

func (payload *ImageCompletionPayload) SetID(id string) {
	payload.ID = id
}

func (payload *ImageCompletionPayload) GetUID() int64 {
	return payload.UID
}

func (payload *ImageCompletionPayload) GetQuota() int64 {
	return payload.Quota
}

func (payload *ImageCompletionPayload) GetModel() string {
	return payload.Model
}

func NewImageCompletionTask(payload any) *asynq.Task {
	data, _ := json.Marshal(payload)
	return asynq.NewTask(TypeImageGenCompletion, data)
}

type ImagePendingTaskPayload struct {
	LeapTaskID string                 `json:"leap_task_id,omitempty"`
	Payload    ImageCompletionPayload `json:"payload,omitempty"`
}

type ImageResponse interface {
	GetID() string
	GetState() string
	IsFinished() bool
	IsProcessing() bool
	UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) ([]string, error)
	GetImages() []string
}

func BuildImageCompletionHandler(
	leapClient *leap.LeapAI,
	stabaiClient *stabilityai.StabilityAI,
	deepaiClient *deepai.DeepAI,
	fromstonClient *fromston.Fromston,
	dashscopeClient *dashscope.DashScope,
	getimgaiClient *getimgai.GetimgAI,
	translator youdao.Translater,
	up *uploader.Uploader,
	rep *repo.Repository,
	oai *openai.OpenAI,
) TaskHandler {
	return func(ctx context.Context, task *asynq.Task) (err error) {
		var payload ImageCompletionPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		if payload.CreatedAt.Add(5 * time.Minute).Before(time.Now()) {
			rep.Queue.Update(context.TODO(), payload.GetID(), repo.QueueTaskStatusFailed, ErrorResult{Errors: []string{"任务处理超时"}})
			log.WithFields(log.Fields{"payload": payload}).Errorf("task expired")
			return nil
		}

		switch payload.Vendor {
		case "leapai":
			return BuildLeapAICompletionHandler(leapClient, translator, up, rep, oai)(ctx, task)
		case "deepai":
			return BuildDeepAICompletionHandler(deepaiClient, translator, up, rep, oai)(ctx, task)
		case "stabilityai":
			return BuildStabilityAICompletionHandler(stabaiClient, translator, up, rep, oai)(ctx, task)
		case "fromston":
			return BuildFromStonCompletionHandler(fromstonClient, up, rep)(ctx, task)
		case "getimgai":
			return BuildGetimgAICompletionHandler(getimgaiClient, translator, up, rep, oai)(ctx, task)
		case "dashscope":
			return BuildDashscopeImageCompletionHandler(dashscopeClient, up, rep, translator, oai)(ctx, task)
		default:
			return nil
		}
	}
}
