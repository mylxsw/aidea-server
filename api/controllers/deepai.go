package controllers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/deepai"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

// DeepAIController DeepAI 控制器
type DeepAIController struct {
	conf       *config.Config
	deepai     *deepai.DeepAI    `autowire:"@"`
	queue      *queue.Queue      `autowire:"@"`
	translater youdao.Translater `autowire:"@"`
}

// NewDeepAIController 创建 DeepAI 控制器
func NewDeepAIController(resolver infra.Resolver, conf *config.Config) web.Controller {
	ctl := DeepAIController{conf: conf}
	resolver.AutoWire(&ctl)

	return &ctl
}

func (ctl *DeepAIController) Register(router web.Router) {
	router.Group("/deepai", func(router web.Router) {
		router.Post("/images/{model}/text-to-image", ctl.imageGenerator)
		router.Post("/images/{model}/text-to-image-async", ctl.imageGeneratorAsync)
	})
}

var deepAIModelNames = array.Map(deepAIModels(), func(m Model, _ int) string {
	return strings.TrimPrefix(m.ID, "deepai:")
})

// imageGenerator 图像生成接口，接口参数参考 https://deepai.org/machine-learning-model/text2img
func (ctl *DeepAIController) imageGenerator(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {

	quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.WithFields(log.Fields{"user_id": user.ID}).Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	model := strings.TrimPrefix(webCtx.PathVar("model"), "deepai:")
	if model == "" {
		return webCtx.JSONError("model is required", http.StatusBadRequest)
	}

	if !array.In(model, deepAIModelNames) {
		return webCtx.JSONError("model is invalid", http.StatusBadRequest)
	}

	text := webCtx.Input("text")
	if text == "" {
		return webCtx.JSONError("text is required", http.StatusBadRequest)
	}

	width := webCtx.IntInput("width", 512)
	height := webCtx.IntInput("height", 512)
	if width > 1536 || height > 1536 || width < 128 || height < 128 {
		return webCtx.JSONError("width/height should between 128 - 1536", http.StatusBadRequest)
	}

	gridSize := webCtx.IntInput("grid_size", 1)
	if gridSize < 1 || gridSize > 4 {
		return webCtx.JSONError("grid_size should between 1 - 4", http.StatusBadRequest)
	}

	negativePrompt := webCtx.InputWithDefault("negative_prompt", "")

	if quota.Quota < quota.Used+coins.GetDeepAIImageCoins(model) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	res, err := ctl.deepai.TextToImage(model, deepai.TextToImageParam{
		Width:        width,
		Height:       height,
		Text:         text,
		GridSize:     gridSize,
		NegativeText: negativePrompt,
	})
	if err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	defer func() {
		if err := quotaRepo.QuotaConsume(ctx, user.ID, coins.GetDeepAIImageCoins(model), repo.NewQuotaUsedMeta("deepai-chat", model)); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}
	}()

	return webCtx.JSON(res)
}

// imageGenerator 图像生成接口，接口参数参考 https://deepai.org/machine-learning-model/text2img
func (ctl *DeepAIController) imageGeneratorAsync(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {

	model := strings.TrimPrefix(webCtx.PathVar("model"), "deepai:")
	if model == "" {
		return webCtx.JSONError("model is required", http.StatusBadRequest)
	}

	if !array.In(model, deepAIModelNames) {
		return webCtx.JSONError("model is invalid", http.StatusBadRequest)
	}

	text := webCtx.Input("text")
	if text == "" {
		return webCtx.JSONError("text is required", http.StatusBadRequest)
	}

	width := webCtx.IntInput("width", 512)
	height := webCtx.IntInput("height", 512)
	if width > 1536 || height > 1536 || width < 128 || height < 128 {
		return webCtx.JSONError("width/height should between 128 - 1536", http.StatusBadRequest)
	}

	gridSize := webCtx.IntInput("grid_size", 1)
	if gridSize < 1 || gridSize > 4 {
		return webCtx.JSONError("grid_size should between 1 - 4", http.StatusBadRequest)
	}

	negativePrompt := webCtx.InputWithDefault("negative_prompt", "")

	quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, err.Error()), http.StatusInternalServerError)
	}

	if quota.Quota < quota.Used+coins.GetDeepAIImageCoins(model) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	// 加入异步任务队列
	taskID, err := ctl.queue.Enqueue(
		&queue.DeepAICompletionPayload{
			Model:          model,
			Quota:          coins.GetDeepAIImageCoins(model),
			UID:            user.ID,
			Prompt:         text,
			NegativePrompt: negativePrompt,
			Width:          int64(width),
			Height:         int64(height),
			ImageCount:     int64(gridSize),
			CreatedAt:      time.Now(),
		},
		queue.NewDeepAICompletionTask,
	)
	if err != nil {
		log.Errorf("enqueue task failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}
	log.WithFields(log.Fields{"task_id": taskID}).Debugf("enqueue task success: %s", taskID)

	return webCtx.JSON(web.M{"task_id": taskID})
}

// DeepAIImageGeneratorResponse 图像生成接口返回结构
type DeepAIImageGeneratorResponse struct {
	ID        string `json:"id"`
	OutputURL string `json:"output_url"`
}
