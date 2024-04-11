package admin

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"net/http"
)

type SettingController struct {
	svc  *service.SettingService `autowire:"@"`
	repo *repo.SettingRepo       `autowire:"@"`
}

func NewSettingController(resolver infra.Resolver) web.Controller {
	ctl := &SettingController{}
	resolver.MustAutoWire(ctl)
	return ctl
}

func (ctl *SettingController) Register(router web.Router) {
	router.Group("/settings", func(router web.Router) {
		router.Get("/", ctl.Settings)
		router.Get("/key/{key}", ctl.Setting)
		router.Post("/key/{key}/reload", ctl.ReloadKey)
		router.Post("/reload", ctl.ReloadAll)
	})
}

// Settings Get all configuration items.
// @Summary Get all configuration items.
// @Tags Admin:Settings
// @Produce json
// @Success 200 {object} common.DataArray[model.Setting]
// @Router /v1/admin/settings [get]
func (ctl *SettingController) Settings(ctx context.Context, webCtx web.Context) web.Response {
	settings, err := ctl.repo.All(ctx)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(common.NewDataArray(settings))
}

// Setting Get the specified configuration item.
// @Summary Get the specified configuration item.
// @Tags Admin:Settings
// @Produce json
// @Param key path string true "Configuration item key"
// @Success 200 {object} common.DataObj[model.Setting]
// @Router /v1/admin/settings/key/{key} [get]
func (ctl *SettingController) Setting(ctx context.Context, webCtx web.Context) web.Response {
	key := webCtx.PathVar("key")
	value, err := ctl.repo.Get(ctx, key)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(common.NewDataObj(value))
}

// ReloadKey Reload the specified configuration item.
// @Summary Reload the specified configuration item.
// @Tags Admin:Settings
// @Produce json
// @Param key path string true "Configuration item key"
// @Success 200 {object} common.EmptyResponse
// @Router /v1/admin/settings/key/{key}/reload [post]
func (ctl *SettingController) ReloadKey(ctx context.Context, webCtx web.Context) web.Response {
	key := webCtx.PathVar("key")
	if err := ctl.svc.ReloadKey(ctx, key); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(common.EmptyResponse{})
}

// ReloadAll Reload all configuration items.
// @Summary Reload all configuration items.
// @Tags Admin:Settings
// @Produce json
// @Success 200 {object} common.EmptyResponse
// @Router /v1/admin/settings/reload [post]
func (ctl *SettingController) ReloadAll(ctx context.Context, webCtx web.Context) web.Response {
	if err := ctl.svc.ReloadAll(ctx); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(common.EmptyResponse{})
}
