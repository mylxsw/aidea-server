package controllers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/stabilityai"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

// StabilityAIController StabilityAI 控制器
type StabilityAIController struct {
	conf       *config.Config
	ai         *stabilityai.StabilityAI `autowire:"@"`
	queue      *queue.Queue             `autowire:"@"`
	translater youdao.Translater        `autowire:"@"`
}

// NewStabilityAIController 创建 StabilityAI 控制器
func NewStabilityAIController(resolver infra.Resolver, conf *config.Config) web.Controller {
	ctl := StabilityAIController{conf: conf}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *StabilityAIController) Register(router web.Router) {
	router.Group("/stabilityai", func(router web.Router) {
		router.Post("/images/{model}/text-to-image", ctl.textToImage)
		router.Post("/images/{model}/text-to-image-async", ctl.textToImageAsync)
	})
}

var stabilityAIModelNames = array.Map(stabilityAIModels(), func(m Model, _ int) string {
	return strings.TrimPrefix(m.ID, "stabilityai:")
})

// TextToImageRequest 文本转图片请求
func (ctl *StabilityAIController) textToImage(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {

	model := strings.TrimPrefix(webCtx.PathVar("model"), "stabilityai:")
	if model == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidModel), http.StatusBadRequest)
	}

	if !array.In(model, stabilityAIModelNames) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidModel), http.StatusBadRequest)
	}

	var r stabilityai.TextToImageRequest
	if err := webCtx.Unmarshal(&r); err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	if len(r.TextPrompts) == 0 {
		return webCtx.JSONError("text_prompts is required", http.StatusBadRequest)
	}

	if r.Steps == 0 {
		r.Steps = 30
	}

	if r.Width == 0 {
		r.Width = 512
	}

	if r.Height == 0 {
		r.Height = 512
	}

	quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.WithFields(log.Fields{"user_id": user.ID}).Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	quotaConsumed := coins.GetStabilityAIImageCoins(model, int64(r.Steps), int64(r.Width), int64(r.Height))
	if quota.Quota < quota.Used+quotaConsumed {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	resp, err := ctl.ai.TextToImage(model, r)
	if err != nil {
		log.Errorf("text to image failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	defer func() {
		if err := quotaRepo.QuotaConsume(ctx, user.ID, quotaConsumed, repo.NewQuotaUsedMeta("stabilityai-chat", model)); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}
	}()

	return webCtx.JSON(resp)
}

func (ctl *StabilityAIController) textToImageAsync(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {
	model := strings.TrimPrefix(webCtx.PathVar("model"), "stabilityai:")
	if model == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidModel), http.StatusBadRequest)
	}

	if !array.In(model, stabilityAIModelNames) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidModel), http.StatusBadRequest)
	}

	var r stabilityai.TextToImageRequest
	if err := webCtx.Unmarshal(&r); err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	if len(r.TextPrompts) == 0 {
		return webCtx.JSONError("text_prompts is required", http.StatusBadRequest)
	}

	if r.Steps == 0 {
		r.Steps = 30
	}

	if r.Width == 0 {
		r.Width = 512
	}

	if r.Height == 0 {
		r.Height = 512
	}

	quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.WithFields(log.Fields{"user_id": user.ID}).Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	quotaConsumed := coins.GetStabilityAIImageCoins(model, int64(r.Steps), int64(r.Width), int64(r.Height))
	if quota.Quota < quota.Used+quotaConsumed {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	text := strings.Join(array.Map(r.TextPrompts, func(p stabilityai.TextPrompts, _ int) string { return p.Text }), "\n")

	// 加入异步任务队列
	taskID, err := ctl.queue.Enqueue(
		&queue.StabilityAICompletionPayload{
			Model:       model,
			Quota:       quotaConsumed,
			UID:         user.ID,
			Prompt:      text,
			Width:       int64(r.Width),
			Height:      int64(r.Height),
			StylePreset: r.StylePreset,
			CreatedAt:   time.Now(),
		},
		queue.NewStabilityAICompletionTask,
	)
	if err != nil {
		log.Errorf("enqueue task failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}
	log.WithFields(log.Fields{"task_id": taskID}).Debugf("enqueue task success: %s", taskID)

	return webCtx.JSON(web.M{"task_id": taskID})
}
