package controllers

import (
	"context"
	"net/http"
	"strings"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
)

// TranslateController 翻译控制器
type TranslateController struct {
	conf       *config.Config
	translater youdao.Translater `autowire:"@"`
}

// NewTranslateController create a new Translate Controller
func NewTranslateController(resolver infra.Resolver, conf *config.Config) web.Controller {
	ctl := &TranslateController{conf: conf}
	resolver.MustAutoWire(ctl)
	return ctl
}

func (ctl *TranslateController) Register(router web.Router) {
	router.Group("/translate", func(router web.Router) {
		router.Post("/", ctl.translate)
	})
}

// Translate 翻译
func (ctl *TranslateController) translate(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo, cacheRepo *repo.CacheRepo) web.Response {
	text := strings.TrimSpace(webCtx.Input("text"))
	if text == "" {
		return webCtx.JSONError("text is required", http.StatusBadRequest)
	}

	quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	quotaConsumed := coins.GetTranslateCoins("youdao", helper.WordCount(text))
	if quota.Quota < quota.Used+quotaConsumed {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	from := strings.TrimSpace(webCtx.InputWithDefault("from", youdao.LanguageAuto))
	target := strings.TrimSpace(webCtx.InputWithDefault("to", common.GetLanguage(webCtx)))

	res, err := ctl.translater.Translate(ctx, from, target, text)
	if err != nil {
		log.Errorf("translate failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	defer func() {
		if err := quotaRepo.QuotaConsume(ctx, user.ID, quotaConsumed, repo.NewQuotaUsedMeta("translate", "youdao")); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}
	}()

	return webCtx.JSON(res)
}
