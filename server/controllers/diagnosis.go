package controllers

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/rate"
	"net/http"
	"time"

	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
)

type DiagnosisController struct {
	limiter *rate.RateLimiter `autowire:"@"`
}

func NewDiagnosisController(resolver infra.Resolver) web.Controller {
	ctl := DiagnosisController{}
	resolver.MustAutoWire(&ctl)

	return &ctl
}

func (ctl *DiagnosisController) Register(router web.Router) {
	router.Group("/diagnosis", func(router web.Router) {
		router.Post("/upload", ctl.uploadDiagnosisInfo)
	})
}

// uploadDiagnosisInfo 上传诊断信息
func (ctl *DiagnosisController) uploadDiagnosisInfo(ctx context.Context, webCtx web.Context, user *auth.UserOptional, info *auth.ClientInfo) web.Response {
	err := ctl.limiter.Allow(ctx, fmt.Sprintf("diagnosis:upload:%s:limit", info.IP), rate.MaxRequestsInPeriod(5, 10*time.Minute))
	if err != nil {
		if err == rate.ErrRateLimitExceeded {
			return webCtx.JSONError("操作频率过高，请稍后再试", http.StatusTooManyRequests)
		}

		return webCtx.JSONError("内部错误，请稍后再试", http.StatusInternalServerError)
	}

	log.WithFields(log.Fields{
		"user":   user.User,
		"client": info,
		"data":   webCtx.Input("data"),
	}).Warning("用户上报诊断信息")

	return webCtx.JSON(web.M{})
}
