package controllers

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	qiniuAuth "github.com/qiniu/go-sdk/v7/auth"
	"github.com/redis/go-redis/v9"
	"net/http"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

// UploadController 文件上传控制器
type UploadController struct {
	conf       *config.Config
	uploader   *uploader.Uploader    `autowire:"@"`
	translater youdao.Translater     `autowire:"@"`
	rds        *redis.Client         `autowire:"@"`
	fs         *repo.FileStorageRepo `autowire:"@"`
}

// NewUploadController 创建文件上传控制器
func NewUploadController(resolver infra.Resolver, conf *config.Config) web.Controller {
	ctl := UploadController{conf: conf}
	resolver.MustAutoWire(&ctl)

	return &ctl
}

func (ctl *UploadController) Register(router web.Router) {
	router.Group("/storage", func(router web.Router) {
		router.Post("/upload-init", ctl.UploadInit)
	})

	router.Group("/callback/storage", func(router web.Router) {
		router.Post("/qiniu", ctl.UploadCallback)
		router.Post("/qiniu-audit", ctl.AuditCallback)
	})
}

// UploadInit 文件上传初始化
func (ctl *UploadController) UploadInit(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {
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
	if usage != "" && !array.In(usage, []string{uploader.UploadUsageAvatar, uploader.UploadUsageChat}) {
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

	expireAfterDays := uploader.DefaultUploadExpireAfterDays
	switch usage {
	case uploader.UploadUsageChat:
		// 聊天图片默认7天过期
		expireAfterDays = 7
	}

	return webCtx.JSON(ctl.uploader.Init(name, int(user.ID), usage, 5, expireAfterDays, true, "client"))
}

type ImageAuditCallback struct {
	// Code 状态码0成功，1等待处理，2正在处理，3处理失败，4通知提交失败
	Code int `json:"code"`
	// Desc 与状态码相对应的详细描述
	Desc string `json:"desc"`
	// InputBucket 处理源文件所在的空间名
	InputBucket string `json:"inputBucket"`
	// InputKey 处理源文件的文件名
	InputKey string `json:"inputKey"`
	// Items 任务处理结果
	Items []ImageAuditCallbackItem `json:"items"`
}

func (cb ImageAuditCallback) IsBlocked() bool {
	for _, item := range cb.Items {
		if item.Result.Result.Suggestion == "block" {
			return true
		}
	}

	return false
}

func (cb ImageAuditCallback) ShouldReview() bool {
	for _, item := range cb.Items {
		if item.Result.Result.Suggestion == "review" {
			return true
		}
	}

	return false
}

func (cb ImageAuditCallback) Labels() []string {
	labels := make([]string, 0)
	for _, item := range cb.Items {
		for _, scene := range item.Result.Result.Scenes {
			if !array.In(scene.Suggestion, []string{"block", "review"}) {
				continue
			}

			label := scene.Result.Label
			if scene.Result.Desc != "" {
				label += "(" + scene.Result.Desc + ")"
			}
			labels = append(labels, label)

		}
	}

	return labels
}

type ImageAuditCallbackItem struct {
	// Code 状态码0成功，1等待处理，2正在处理，3处理失败，4通知提交失败
	Code int `json:"code"`
	// Desc 与状态码相对应的详细描述
	Desc string `json:"desc"`
	// Result 任务处理结果
	Result ImageAuditCallbackItemResult `json:"result"`
}

type ImageAuditCallbackItemResult struct {
	Disable bool                               `json:"disable"`
	Result  ImageAuditCallbackItemResultResult `json:"result"`
}

type ImageAuditCallbackItemResultResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	// Suggestion 图片的管控建议值，取值包括：[“block”,”review”,”pass”]
	Suggestion string `json:"suggestion"`
	// Scenes 图片的审核结果，包括政治敏感识别结果和色情识别结果
	// - pulp 是涉黄识别的检测结果
	// - terror 是暴恐识别的检测结果
	// - politician 是敏感人物识别的检测结果
	Scenes map[string]Scene `json:"scenes"`
}

type Scene struct {
	Result SceneResult `json:"result"`
	// Suggestion 图片的管控建议值，取值包括：[“block”,”review”,”pass”]
	Suggestion string `json:"suggestion"`
}

type SceneResult struct {
	Label string  `json:"label,omitempty"`
	Desc  string  `json:"desc,omitempty"`
	Score float64 `json:"score,omitempty"`
}

// AuditCallback 文件审核回调（七牛云发起）
func (ctl *UploadController) AuditCallback(ctx context.Context, webCtx web.Context) web.Response {
	var ret ImageAuditCallback
	if err := webCtx.Unmarshal(&ret); err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, err.Error()), http.StatusBadRequest)
	}

	if ret.IsBlocked() {
		blockedURL := ctl.uploader.MakePrivateURL(ret.InputKey, time.Second*3600*24)

		key := misc.UUID()
		if err := ctl.rds.Set(ctx, fmt.Sprintf("redirect:%s", key), blockedURL, time.Second*3600*24).Err(); err != nil {
			log.F(log.M{"blocked_url": blockedURL}).Errorf("set redirect url failed: %s", err)
		}

		log.With(ret).Errorf(
			"图片包含违禁内容：%s。\n\n[访问地址](%s)",
			strings.Join(ret.Labels(), "|"),
			fmt.Sprintf("%s/public/r/%s", ctl.conf.BaseURL, key),
		)

		err := ctl.fs.UpdateByKey(ctx, ret.InputKey, repo.StorageFileStatusDisabled, strings.Join(ret.Labels(), "|"))
		if err != nil {
			log.With(ret).Errorf("update file status failed: %s", err)
		}
	} else if ret.ShouldReview() {
		err := ctl.fs.UpdateByKey(ctx, ret.InputKey, repo.StorageFileStatusReview, strings.Join(ret.Labels(), "|"))
		if err != nil {
			log.With(ret).Errorf("update file status failed: %s", err)
		}
	}

	return webCtx.JSON(web.M{})
}

// UploadCallback 文件上传回调（七牛云发起）
func (ctl *UploadController) UploadCallback(ctx context.Context, webCtx web.Context, quotaRepo *repo.QuotaRepo) web.Response {
	// 验证请求来源为七牛云
	mac := qiniuAuth.New(ctl.conf.StorageAppKey, ctl.conf.StorageAppSecret)
	if ok, err := mac.VerifyCallback(webCtx.Request().Raw()); err != nil || !ok {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "invalid callback"), http.StatusBadRequest)
	}

	var cb uploader.UploadCallback
	if err := webCtx.Unmarshal(&cb); err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, err.Error()), http.StatusBadRequest)
	}

	log.With(cb).Debugf("upload callback")

	if err := quotaRepo.QuotaConsume(ctx, cb.UID, coins.GetUploadCoins(), repo.NewQuotaUsedMeta("upload", "qiniu")); err != nil {
		log.With(cb).Errorf("used quota add failed: %s", err)
	}

	_, err := ctl.fs.Save(ctx, model.StorageFile{
		UserId:   cb.UID,
		Name:     cb.Name,
		Hash:     cb.Hash,
		FileKey:  cb.Key,
		FileSize: cb.Fsize,
		Bucket:   cb.Bucket,
		Status:   repo.StorageFileStatusEnabled,
		Channel:  cb.Channel,
	})
	if err != nil {
		log.With(cb).Errorf("save file info failed: %s", err)
	}

	return webCtx.JSON(web.M{})
}
