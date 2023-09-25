package v2

import (
	"context"
	"net/http"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

type RoomController struct {
	conf       *config.Config    `autowire:"@"`
	roomRepo   *repo.RoomRepo    `autowire:"@"`
	translater youdao.Translater `autowire:"@"`
}

func NewRoomController(resolver infra.Resolver) web.Controller {
	ctl := RoomController{}
	resolver.MustAutoWire(&ctl)

	return &ctl
}

func (ctl *RoomController) Register(router web.Router) {
	router.Group("/rooms", func(router web.Router) {
		router.Get("/", ctl.Rooms)
	})
}

const RoomsQueryLimit = 100

// Rooms 获取房间列表
func (ctl *RoomController) Rooms(ctx context.Context, webCtx web.Context, user *auth.User, client *auth.ClientInfo) web.Response {
	rooms, err := ctl.roomRepo.Rooms(ctx, user.ID, RoomsQueryLimit)
	if err != nil {
		log.F(log.M{"user_id": user.ID}).Errorf("查询用户房间列表失败: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	var suggests []repo.GalleryRoom
	if len(rooms) == 0 {
		suggests, err = ctl.roomRepo.GallerySuggests(ctx, 11)
		if err != nil {
			log.Errorf("查询推荐房间列表失败: %v", err)
			// 注意：这里不返回错误，因为推荐房间列表不是必须的
		}

		cnLocalMode := client.IsCNLocalMode(ctl.conf)
		suggests = array.Filter(suggests, func(item repo.GalleryRoom, _ int) bool {
			// 如果启用了国产化模式，则过滤掉 openai 和 Anthropic 的模型
			if cnLocalMode && array.In(item.Vendor, []string{"openai", "Anthropic"}) {
				return false
			}

			// 检查模型是否满足条件
			if !ctl.conf.EnableOpenAI && item.Vendor == "openai" {
				return false
			}

			if !ctl.conf.EnableBaiduWXAI && item.Vendor == "文心千帆" {
				return false
			}

			if !ctl.conf.EnableDashScopeAI && item.Vendor == "灵积" {
				return false
			}

			if !ctl.conf.EnableXFYunAI && item.Vendor == "讯飞星火" {
				return false
			}

			if !ctl.conf.EnableSenseNovaAI && item.Vendor == "商汤日日新" {
				return false
			}

			if !ctl.conf.EnableTencentAI && item.Vendor == "腾讯" {
				return false
			}

			if !ctl.conf.EnableAnthropic && item.Vendor == "Anthropic" {
				return false
			}

			if item.VersionMax == "" && item.VersionMin == "" {
				return true
			}

			if item.VersionMin != "" && helper.VersionOlder(client.Version, item.VersionMin) {
				return false
			}

			if item.VersionMax != "" && helper.VersionNewer(client.Version, item.VersionMax) {
				return false
			}

			return true
		})
	}

	return webCtx.JSON(web.M{
		"data":     rooms,
		"suggests": suggests,
	})
}
