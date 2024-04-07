package controllers

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/glacier/infra"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

// ModelController 模型控制器
type ModelController struct {
	conf *config.Config   `autowire:"@"`
	svc  *service.Service `autowire:"@"`
}

// NewModelController 创建模型控制器
func NewModelController(resolver infra.Resolver) web.Controller {
	ctl := &ModelController{}
	resolver.MustAutoWire(ctl)

	return ctl
}

func (ctl *ModelController) Register(router web.Router) {
	router.Group("/models", func(router web.Router) {
		router.Get("/", ctl.Models)
	})
}

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ShortName   string `json:"short_name"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Category    string `json:"category"`
	IsImage     bool   `json:"is_image"`
	Disabled    bool   `json:"disabled"`
	VersionMin  string `json:"version_min,omitempty"`
	VersionMax  string `json:"version_max,omitempty"`
	Tag         string `json:"tag,omitempty"`

	IsChat        bool `json:"is_chat"`
	SupportVision bool `json:"support_vision,omitempty"`
}

// Models 获取模型列表
func (ctl *ModelController) Models(ctx context.Context, webCtx web.Context, client *auth.ClientInfo, user *auth.UserOptional) web.Response {
	models := array.Map(ctl.svc.Chat.Models(ctx, true), func(item repo.Model, _ int) Model {
		ret := Model{
			ID:            item.ModelId,
			Name:          item.Name,
			ShortName:     item.ShortName,
			Description:   item.Description,
			AvatarURL:     item.AvatarUrl,
			Category:      "",
			IsImage:       false,
			Disabled:      item.Status == repo.ModelStatusDisabled,
			VersionMin:    item.VersionMin,
			VersionMax:    item.VersionMax,
			IsChat:        true,
			SupportVision: item.Meta.Vision,
		}

		if ret.Disabled {
			return ret
		}

		if client.Version != "" && item.VersionMin != "" && misc.VersionOlder(client.Version, item.VersionMin) {
			ret.Disabled = true
			return ret
		}

		if client.Version != "" && item.VersionMax != "" && misc.VersionNewer(client.Version, item.VersionMax) {
			ret.Disabled = true
			return ret
		}

		if client.IsCNLocalMode(ctl.conf) && item.Meta.Restricted && (user.User == nil || !user.User.ExtraPermissionUser()) {
			ret.Disabled = true
			return ret
		}

		return ret
	})

	return webCtx.JSON(models)
}
