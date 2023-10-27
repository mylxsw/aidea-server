package controllers

import (
	"github.com/mylxsw/aidea-server/internal/ai/chat"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/helper"
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
	if helper.VersionNewer(client.Version, "1.0.6") {
		models := array.Map(chat.Models(ctl.conf, true), func(item chat.Model, _ int) chat.Model {
			if item.Disabled {
				return item
			}

			if item.VersionMin != "" && helper.VersionOlder(client.Version, item.VersionMin) {
				item.Disabled = true
				return item
			}

			if item.VersionMax != "" && helper.VersionNewer(client.Version, item.VersionMax) {
				item.Disabled = true
				return item
			}

			if client.IsCNLocalMode(ctl.conf) && item.IsSenstiveModel() {
				item.Disabled = true
				return item
			}

			return item
		})

		return ctx.JSON(models)
	}

	models := array.Filter(chat.Models(ctl.conf, false), func(item chat.Model, _ int) bool {
		if item.VersionMin != "" && helper.VersionOlder(client.Version, item.VersionMin) {
			return false
		}

		if item.VersionMax != "" && helper.VersionNewer(client.Version, item.VersionMax) {
			return false
		}

		return !(client.IsCNLocalMode(ctl.conf) && item.IsSenstiveModel())
	})

	return ctx.JSON(models)
}
