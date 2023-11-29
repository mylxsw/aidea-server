package controllers

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type APIKeyController struct {
	repo *repo.Repository `autowire:"@"`
}

func NewAPIKeyController(resolver infra.Resolver) web.Controller {
	ctl := &APIKeyController{}
	resolver.MustAutoWire(ctl)

	return ctl
}

func (ctl *APIKeyController) Register(router web.Router) {
	router.Group("/api-keys", func(router web.Router) {
		router.Get("/", ctl.List)
		router.Post("/", ctl.Create)
		router.Get("/{id}", ctl.GetKey)
		router.Delete("/{id}", ctl.Delete)
	})
}

// List API Key 列表
func (ctl *APIKeyController) List(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	keys, err := ctl.repo.User.GetAPIKeys(ctx, user.ID)
	if err != nil {
		return webCtx.JSONError(common.ErrInternalError, http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"data": keys})
}

// GetKey 获取 API Key 详情
func (ctl *APIKeyController) GetKey(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	keyID, _ := strconv.Atoi(webCtx.PathVar("id"))
	if keyID <= 0 {
		return webCtx.JSONError(common.ErrInvalidRequest, http.StatusBadRequest)
	}

	key, err := ctl.repo.User.GetAPIKey(ctx, user.ID, int64(keyID))
	if err != nil {
		return webCtx.JSONError(common.ErrInternalError, http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"data": key})
}

// Create 创建 API Key
func (ctl *APIKeyController) Create(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	name := strings.TrimSpace(webCtx.Input("name"))
	if name == "" {
		name = "Default"
	}

	key, err := ctl.repo.User.CreateAPIKey(ctx, user.ID, name, time.Now().AddDate(1, 0, 0))
	if err != nil {
		log.Errorf("create api key failed: %v", err)
		return webCtx.JSONError(common.ErrInternalError, http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"key": key})
}

// Delete 删除 API Key
func (ctl *APIKeyController) Delete(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	keyID, _ := strconv.Atoi(webCtx.PathVar("id"))
	if keyID <= 0 {
		return webCtx.JSONError(common.ErrInvalidRequest, http.StatusBadRequest)
	}

	if err := ctl.repo.User.DeleteAPIKey(ctx, user.ID, int64(keyID)); err != nil {
		return webCtx.JSONError(common.ErrInternalError, http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}
