package controllers

import (
	"context"
	"net/http"
	"strings"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

// UploadController 文件上传控制器
type UploadController struct {
	conf       *config.Config
	uploader   *uploader.Uploader `autowire:"@"`
	translater youdao.Translater  `autowire:"@"`
}

// NewUploadController 创建文件上传控制器
func NewUploadController(resolver infra.Resolver, conf *config.Config) web.Controller {
	ctl := UploadController{conf: conf}
	resolver.AutoWire(&ctl)

	return &ctl
}

func (ctl *UploadController) Register(router web.Router) {
	router.Group("/storage", func(router web.Router) {
		router.Post("/upload-init", ctl.uploadInit)
	})

	router.Group("/callback/storage", func(router web.Router) {
		router.Post("/qiniu", ctl.uploadCallback)
	})
}

// uploadInit 文件上传初始化
func (ctl *UploadController) uploadInit(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {
	filesize := webCtx.IntInput("filesize", 0)
	if filesize <= 0 || filesize > 1024*1024*5 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "文件大小不能超过 5M"), http.StatusBadRequest)
	}

	name := webCtx.Input("name")
	if name == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "文件名不能为空"), http.StatusBadRequest)
	}

	nameSeg := strings.Split(name, ".")
	if len(nameSeg) < 2 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "文件名格式不正确，必须包含扩展名"), http.StatusBadRequest)
	}

	if !array.In(strings.ToLower(nameSeg[len(nameSeg)-1]), []string{"jpg", "jpeg", "png", "gif"}) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "文件格式不正确，仅支持 jpg/jpeg/png/gif"), http.StatusBadRequest)
	}

	usage := webCtx.Input("usage")
	if usage != "" && !array.In(usage, []string{uploader.UploadUsageAvatar}) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "文件用途不正确"), http.StatusBadRequest)
	}

	quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if quota.Quota < quota.Used+coins.GetUploadCoins() {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	return webCtx.JSON(ctl.uploader.Init(name, int(user.ID), usage, 5, uploader.DefaultUploadExpireAfterDays, true))
}

// uploadCallback 文件上传回调（七牛云发起）
func (ctl *UploadController) uploadCallback(ctx context.Context, webCtx web.Context, quotaRepo *repo.QuotaRepo) web.Response {
	var cb UploadCallback
	if err := webCtx.Unmarshal(&cb); err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, err.Error()), http.StatusBadRequest)
	}

	log.With(cb).Debugf("upload callback")

	if err := quotaRepo.QuotaConsume(ctx, cb.UID, coins.GetUploadCoins(), repo.NewQuotaUsedMeta("upload", "qiniu")); err != nil {
		log.With(cb).Errorf("used quota add failed: %s", err)
	}

	return webCtx.JSON(web.M{})
}

type UploadCallback struct {
	Key    string `json:"key"`
	Hash   string `json:"hash"`
	Fsize  int64  `json:"fsize"`
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
	UID    int64  `json:"uid"`
}
