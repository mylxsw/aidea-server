package controllers

import (
	"context"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/glacier/infra"
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
	ID           string `json:"id"`
	Name         string `json:"name"`
	ShortName    string `json:"short_name"`
	Description  string `json:"description"`
	PriceInfo    string `json:"price_info,omitempty"`
	AvatarURL    string `json:"avatar_url,omitempty"`
	Category     string `json:"category"`
	IsImage      bool   `json:"is_image"`
	Disabled     bool   `json:"disabled"`
	VersionMin   string `json:"version_min,omitempty"`
	VersionMax   string `json:"version_max,omitempty"`
	Tag          string `json:"tag,omitempty"`
	TagTextColor string `json:"tag_text_color,omitempty"`
	TagBgColor   string `json:"tag_bg_color,omitempty"`
	IsNew        bool   `json:"is_new"`

	IsChat        bool `json:"is_chat"`
	SupportVision bool `json:"support_vision,omitempty"`
	IsDefault     bool `json:"is_default,omitempty"`
	Recommend     bool `json:"recommend,omitempty"`
}

// Models 获取模型列表
func (ctl *ModelController) Models(ctx context.Context, webCtx web.Context, client *auth.ClientInfo, user *auth.UserOptional) web.Response {
	models := array.Map(ctl.svc.Chat.Models(ctx, true), func(item repo.Model, _ int) Model {
		ret := Model{
			ID:            item.ModelId,
			Name:          item.Name,
			ShortName:     item.ShortName,
			Description:   "",
			AvatarURL:     item.AvatarUrl,
			Category:      item.Meta.Category,
			IsImage:       false,
			Disabled:      item.Status == repo.ModelStatusDisabled,
			VersionMin:    item.VersionMin,
			VersionMax:    item.VersionMax,
			IsChat:        true,
			SupportVision: item.Meta.Vision,
			IsNew:         item.Meta.IsNew,
			Tag:           item.Meta.Tag,
			TagTextColor:  item.Meta.TagTextColor,
			TagBgColor:    item.Meta.TagBgColor,
		}

		if item.Meta.InputPrice == 0 && item.Meta.OutputPrice == 0 && item.Meta.PerReqPrice == 0 {
			ret.Tag = "限免"
			ret.TagTextColor = "FFFFFFFF"
			ret.TagBgColor = "FF2196F3"
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

	sortPriority := []string{"OpenAI", "Anthropic", "Google"}
	models = array.Sort(models, func(i, j Model) bool {
		if i.Category == "" && j.Category != "" {
			return false
		} else if i.Category != "" && j.Category == "" {
			return true
		}

		if i.Category == j.Category {
			return i.Name < j.Name
		}

		ii := misc.IndexOf(sortPriority, i.Category)
		ji := misc.IndexOf(sortPriority, j.Category)

		if ii != -1 && ji == -1 {
			return true
		}

		if ii == -1 && ji != -1 {
			return false
		}

		if ii != -1 && ji != -1 {
			return ii < ji
		}

		return i.Category < j.Category
	})

	return webCtx.JSON(models)
}
