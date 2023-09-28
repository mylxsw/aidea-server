package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
)

type CreativeIslandController struct {
	trans        youdao.Translater  `autowire:"@"`
	creativeRepo *repo.CreativeRepo `autowire:"@"`
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

	return webCtx.JSON(web.M{
		"message": "success",
	})
}
