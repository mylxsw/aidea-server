package v2

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/array"
	"net/http"
	"strconv"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/web"
)

// ModelController 模型控制器
type ModelController struct {
	conf    *config.Config       `autowire:"@"`
	repo    *repo.Repository     `autowire:"@"`
	userSrv *service.UserService `autowire:"@"`
}

// NewModelController 创建模型控制器
func NewModelController(resolver infra.Resolver) web.Controller {
	ctl := &ModelController{}
	resolver.MustAutoWire(ctl)
	return ctl
}

func (ctl *ModelController) Register(router web.Router) {
	router.Group("/models", func(router web.Router) {
		// 获取模型支持的风格
		router.Get("/styles", ctl.Styles)
		// 自定义首页模型
		router.Get("/home-models/all", ctl.GetAllHomeModels)
		router.Get("/home-models/{key}", ctl.GetHomeModelsItem)
	})
}

type ModelStyle struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Preview string `json:"preview,omitempty"`
}

func (ctl *ModelController) Styles(ctx context.Context, webCtx web.Context) web.Response {
	return webCtx.JSON([]ModelStyle{
		{ID: "enhance", Name: "效果增强", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/enhance.png-square_500"},
		{ID: "anime", Name: "日本动漫", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/anime.png-square_500"},
		{ID: "photographic", Name: "摄影", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/photographic.png-square_500"},
		{ID: "digital-art", Name: "数字艺术", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/digital-art.png-square_500"},
		{ID: "comic-book", Name: "漫画书", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/comic-book.png-square_500"},
		{ID: "fantasy-art", Name: "奇幻艺术", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/fantasy-art.png-square_500"},
		{ID: "analog-film", Name: "模拟电影", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/analog-film.png-square_500"},
		{ID: "neon-punk", Name: "赛博朋克", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/neon-punk.png-square_500"},
		{ID: "isometric", Name: "等距视角", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/isometric.png-square_500"},
		{ID: "low-poly", Name: "低多边形", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/low-poly.png-square_500"},
		{ID: "origami", Name: "折纸", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/origami.png-square_500"},
		{ID: "line-art", Name: "线条艺术", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/line-art.png-square_500"},
		{ID: "modeling-compound", Name: "粘土工艺", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/modeling-compound.png-square_500"},
		{ID: "cinematic", Name: "电影风格", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/cinematic.png-square_500"},
		{ID: "3d-model", Name: "3D 模型", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/3d-model.png-square_500"},
		{ID: "pixel-art", Name: "像素艺术", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/pixel-art.png-square_500"},
	})
}

// GetAllHomeModels 获取所有首页模型
func (ctl *ModelController) GetAllHomeModels(ctx context.Context, webCtx web.Context, user *auth.UserOptional) web.Response {
	homeModels := make([]service.HomeModel, 0)

	models := chat.Models(ctl.conf, true)
	modelIDMap := array.ToMap(models, func(item chat.Model, _ int) string {
		return item.RealID()
	})

	// 类型：model
	homeModels = append(
		homeModels,
		array.Map(models, func(item chat.Model, _ int) service.HomeModel {
			return service.HomeModel{
				Type:          service.HomeModelTypeModel,
				ID:            item.ID,
				Name:          item.Name,
				ModelID:       item.ID,
				ModelName:     item.Name,
				AvatarURL:     item.AvatarURL,
				SupportVision: item.SupportVision,
			}
		})...,
	)

	// 类型：room_gallery
	galleries, err := ctl.repo.Room.Galleries(ctx)
	if err != nil {
		log.Errorf("get room galleries failed: %v", err)
	}
	homeModels = append(
		homeModels,
		array.Map(galleries, func(item repo.GalleryRoom, _ int) service.HomeModel {
			model := modelIDMap[item.Model]
			return service.HomeModel{
				Type:          service.HomeModelTypeRoomGallery,
				ID:            strconv.Itoa(int(item.Id)),
				Name:          item.Name,
				AvatarURL:     item.AvatarUrl,
				ModelID:       model.ID,
				ModelName:     model.Name,
				SupportVision: model.SupportVision,
			}
		})...,
	)

	// 类型：rooms
	if user.User != nil {
		roomTypes := []int{repo.RoomTypePreset, repo.RoomTypeCustom, repo.RoomTypePresetCustom}
		rooms, err := ctl.repo.Room.Rooms(ctx, user.User.ID, roomTypes, 500)
		if err != nil {
			log.WithFields(log.Fields{"user_id": user.User.ID}).Errorf("get rooms failed: %v", err)
		}
		homeModels = append(
			homeModels,
			array.Map(rooms, func(item repo.Room, _ int) service.HomeModel {
				model := modelIDMap[item.Model]
				return service.HomeModel{
					Type:          service.HomeModelTypeRooms,
					ID:            strconv.Itoa(int(item.Id)),
					Name:          item.Name,
					AvatarURL:     item.AvatarUrl,
					ModelID:       model.ID,
					ModelName:     model.Name,
					SupportVision: model.SupportVision,
				}
			})...,
		)
	}

	return webCtx.JSON(web.M{
		"data": homeModels,
	})
}

// GetHomeModelsItem 获取所有首页模型详情
func (ctl *ModelController) GetHomeModelsItem(ctx context.Context, webCtx web.Context, user *auth.UserOptional) web.Response {
	var userID int64
	if user.User != nil {
		userID = user.User.ID
	}

	key := webCtx.PathVar("key")

	models := array.ToMap(chat.Models(ctl.conf, true), func(item chat.Model, _ int) string {
		return item.RealID()
	})
	homeModel, err := ctl.userSrv.QueryHomeModel(ctx, models, userID, key)
	if err != nil {
		log.WithFields(log.Fields{"key": key, "models": models}).Errorf("query home model failed: %v", err)
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data": homeModel,
	})
}
