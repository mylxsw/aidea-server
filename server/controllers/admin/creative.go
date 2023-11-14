package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
)

type CreativeIslandController struct {
	conf         *config.Config     `autowire:"@"`
	trans        youdao.Translater  `autowire:"@"`
	creativeRepo *repo.CreativeRepo `autowire:"@"`
	uploader     *uploader.Uploader `autowire:"@"`
}

func NewCreativeIslandController(resolver infra.Resolver) web.Controller {
	ctl := CreativeIslandController{}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *CreativeIslandController) Register(router web.Router) {
	router.Group("/creative-island", func(router web.Router) {
		router.Put("/histories/{id}/forbid", ctl.ForbidCreativeHistory)
	})
}

// ForbidCreativeHistory 违规创作岛历史纪录封禁
func (ctl *CreativeIslandController) ForbidCreativeHistory(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	historyID, err := strconv.Atoi(webCtx.PathVar("id"))
	if err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrNotFound), http.StatusNotFound)
	}

	item, err := ctl.creativeRepo.FindHistoryRecord(ctx, 0, int64(historyID))
	if err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	if item.Status == int64(repo.CreativeStatusForbid) {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, "当前记录已封禁，请勿重复操作"), http.StatusBadRequest)
	}

	if err := ctl.creativeRepo.UpdateRecordStatusByID(ctx, int64(historyID), fmt.Sprintf("内容违规\n%s", item.Answer), repo.CreativeStatusForbid); err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	// 禁用文件（arguments->image，answer)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.WithFields(log.Fields{
					"history_id": historyID,
				}).Errorf("禁用文件失败: %v", err)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		urls := make([]string, 0)

		var answers []string
		_ = json.Unmarshal([]byte(item.Answer), &answers)
		for _, answer := range answers {
			if err := ctl.uploader.ForbidFile(ctx, strings.TrimPrefix(answer, strings.TrimSuffix(ctl.conf.StorageDomain, "/")+"/")); err != nil {
				log.WithFields(log.Fields{"file": answer}).Errorf("禁用文件失败: %v", err)
			}

			urls = append(urls, answer)
		}

		var arguments map[string]any
		_ = json.Unmarshal([]byte(item.Arguments), &arguments)
		if image, ok := arguments["image"]; ok {
			if err := ctl.uploader.ForbidFile(ctx, strings.TrimPrefix(image.(string), strings.TrimSuffix(ctl.conf.StorageDomain, "/")+"/")); err != nil {
				log.WithFields(log.Fields{"file": image}).Errorf("禁用文件失败: %v", err)
			}
			urls = append(urls, image.(string))
		}

		if len(urls) > 0 {
			if _, err := ctl.uploader.RefreshCDN(ctx, urls); err != nil {
				log.WithFields(log.Fields{"urls": urls}).Errorf("清空 CDN 缓存失败: %v", err)
			}
		}
	}()

	return webCtx.JSON(web.M{
		"message": "success",
	})
}
