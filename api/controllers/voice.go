package controllers

import (
	"context"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/voice"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

type VoiceController struct {
	voice      *voice.Voice      `autowire:"@"`
	translater youdao.Translater `autowire:"@"`
	quotaRepo  *repo.QuotaRepo   `autowire:"@"`
}

func NewVoiceController(resolver infra.Resolver) web.Controller {
	ctl := &VoiceController{}
	resolver.MustAutoWire(ctl)
	return ctl
}

func (ctl *VoiceController) Register(router web.Router) {
	router.Group("/voice", func(router web.Router) {
		router.Post("/text2voice", ctl.Text2Voice)
	})
}

// Text2Voice 语音合成
func (ctl *VoiceController) Text2Voice(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	text := strings.TrimSpace(webCtx.Input("text"))
	if text == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "语音文本不能为空"), http.StatusBadRequest)
	}

	style := webCtx.Int64Input("style", 7)

	// 把 text 以 200 个字符为单位分割
	segments := helper.TextSplit(text, 200)

	// 优先检查缓存中是否存在之前生成的结果，每一段全部符合则返回，不再扣费
	cachedResults := array.Filter(
		array.Map(segments, func(segment string, _ int) string {
			res, _ := ctl.voice.Text2VoiceOnlyCached(ctx, style, segment)
			return res
		}),
		func(result string, _ int) bool { return result != "" },
	)
	if len(cachedResults) == len(segments) {
		return webCtx.JSON(web.M{
			"results": cachedResults,
		})
	}

	// 每 3 个 segment 一次性收费
	payCount := int64(math.Ceil(float64(len(segments)) / 3.0))

	quota, err := ctl.quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if quota.Quota < quota.Used+coins.GetTextToVoiceCoins()*payCount {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	var wg sync.WaitGroup
	wg.Add(len(segments))

	results := make([]string, len(segments))
	for idx, segment := range segments {
		go func(idx int, segment string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			result, err := ctl.voice.Text2VoiceCached(ctx, style, segment)
			if err != nil {
				log.Errorf("text to voice failed: %s", err)
				return
			}

			results[idx] = result
		}(idx, segment)
	}

	wg.Wait()

	// 扣除用户的配额
	if err := ctl.quotaRepo.QuotaConsume(ctx, user.ID, coins.GetTextToVoiceCoins()*payCount, repo.NewQuotaUsedMeta("text2voice", "qiniu")); err != nil {
		log.WithFields(log.Fields{
			"result":  results,
			"user_id": user.ID,
		}).Errorf("used quota add failed for text to voice: %s", err)
	}

	return webCtx.JSON(web.M{
		"results": results,
	})
}
