package controllers

import (
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/misc"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

// ModelController 模型控制器
type ModelController struct {
	conf *config.Config
}

// NewModelController 创建模型控制器
func NewModelController(conf *config.Config) web.Controller {
	return &ModelController{conf: conf}
}

func (ctl *ModelController) Register(router web.Router) {
	router.Group("/models", func(router web.Router) {
		router.Get("/", ctl.Models)
	})
}

// Models 获取模型列表
func (ctl *ModelController) Models(ctx web.Context, client *auth.ClientInfo) web.Response {
	if client.Version == "" || misc.VersionNewer(client.Version, "1.0.6") {
		models := array.Map(chat.Models(ctl.conf, true), func(item chat.Model, _ int) chat.Model {
			if item.Disabled {
				return item
			}

			if client.Version != "" && item.VersionMin != "" && misc.VersionOlder(client.Version, item.VersionMin) {
				item.Disabled = true
				return item
			}

			if client.Version != "" && item.VersionMax != "" && misc.VersionNewer(client.Version, item.VersionMax) {
				item.Disabled = true
				return item
			}

			if client.IsCNLocalMode(ctl.conf) && item.IsSensitiveModel() {
				item.Disabled = true
				return item
			}

			return item
		})

		return ctx.JSON(models)
	}

	models := array.Filter(chat.Models(ctl.conf, false), func(item chat.Model, _ int) bool {
		if item.VersionMin != "" && misc.VersionOlder(client.Version, item.VersionMin) {
			return false
		}

		if item.VersionMax != "" && misc.VersionNewer(client.Version, item.VersionMax) {
			return false
		}

		return !(client.IsCNLocalMode(ctl.conf) && item.IsSensitiveModel())
	})

	return ctx.JSON(models)
}
