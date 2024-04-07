package v2

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"net/http"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

type RoomController struct {
	conf       *config.Config    `autowire:"@"`
	roomRepo   *repo.RoomRepo    `autowire:"@"`
	translater youdao.Translater `autowire:"@"`
	svc        *service.Service  `autowire:"@"`
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
	roomTypes := []int{repo.RoomTypePreset, repo.RoomTypePresetCustom, repo.RoomTypeCustom}
	if misc.VersionNewer(client.Version, "1.0.6") {
		roomTypes = append(roomTypes, repo.RoomTypeGroupChat)
	}

	rooms, err := ctl.roomRepo.Rooms(ctx, user.ID, roomTypes, RoomsQueryLimit)
	if err != nil {
		log.F(log.M{"user_id": user.ID}).Errorf("查询用户房间列表失败: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	models := array.ToMap(
		ctl.svc.Chat.Models(ctx, true),
		func(item repo.Model, _ int) string { return item.ModelId },
	)

	var suggests []repo.GalleryRoom
	if len(rooms) == 0 {
		suggests, err = ctl.roomRepo.GallerySuggests(ctx, 11)
		if err != nil {
			log.Errorf("查询推荐房间列表失败: %v", err)
			// 注意：这里不返回错误，因为推荐房间列表不是必须的
		}

		cnLocalMode := client.IsCNLocalMode(ctl.conf) && !user.ExtraPermissionUser()
		suggests = array.Filter(suggests, func(item repo.GalleryRoom, _ int) bool {
			mod, ok := models[item.Model]
			if !ok {
				return false
			}

			// 如果启用了国产化模式，则过滤掉 openai 和 Anthropic 的模型
			if cnLocalMode && item.RoomType == "system" && mod.Meta.Restricted {
				return false
			}

			if mod.Status == repo.ModelStatusDisabled {
				return false
			}

			if item.VersionMax == "" && item.VersionMin == "" {
				return true
			}

			if item.VersionMin != "" && misc.VersionOlder(client.Version, item.VersionMin) {
				return false
			}

			if item.VersionMax != "" && misc.VersionNewer(client.Version, item.VersionMax) {
				return false
			}

			return true
		})
	}

	return webCtx.JSON(web.M{
		"data": array.Map(rooms, func(item repo.Room, _ int) repo.Room {
			// 替换成员列表为头像列表
			members := make([]string, 0)
			if len(item.Members) > 0 {
				for _, member := range item.Members {
					if mod, ok := models[member]; ok && mod.AvatarUrl != "" {
						members = append(members, mod.AvatarUrl)
					}
				}

				item.Members = members
			}

			if item.AvatarUrl == "" {
				if mod, ok := models[item.Model]; ok && mod.AvatarUrl != "" {
					item.AvatarUrl = mod.AvatarUrl
				}
			}

			return item
		}),
		"suggests": suggests,
	})
}
