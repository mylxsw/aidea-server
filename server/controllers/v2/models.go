package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/ternary"
	"net/http"
	"strconv"
	"strings"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/web"
)

// ModelController 模型控制器
type ModelController struct {
	conf    *config.Config       `autowire:"@"`
	repo    *repo.Repository     `autowire:"@"`
	userSrv *service.UserService `autowire:"@"`
	svc     *service.Service     `autowire:"@"`
}

// NewModelController 创建模型控制器
func NewModelController(resolver infra.Resolver) web.Controller {
	ctl := &ModelController{}
	resolver.MustAutoWire(ctl)
	return ctl
}

func (ctl *ModelController) Register(router web.Router) {
	router.Group("/models", func(router web.Router) {
		// Get all models
		router.Get("/", ctl.Models)
		// 获取模型支持的风格
		router.Get("/styles", ctl.Styles)
		// 自定义首页模型
		router.Get("/home-models/all", ctl.GetAllHomeModels)
		router.Get("/home-models/{key}", ctl.GetHomeModelsItem)
	})
}

// Models Loading all the models, including custom digital humans in the v2-release
func (ctl *ModelController) Models(ctx context.Context, webCtx web.Context, client *auth.ClientInfo, user *auth.UserOptional) web.Response {
	models := ctl.loadRawModels(ctx, client, user)

	withCustom := webCtx.Input("with-custom")
	if withCustom == "true" && user.User != nil {
		roomTypes := []int{repo.RoomTypePreset, repo.RoomTypeCustom, repo.RoomTypePresetCustom}
		rooms, err := ctl.repo.Room.Rooms(ctx, user.User.ID, roomTypes, 500)
		if err != nil {
			log.WithFields(log.Fields{"user_id": user.User.ID}).Errorf("get rooms failed: %v", err)
		}

		modelIDMap := array.ToMap(models, func(item controllers.Model, _ int) string {
			return item.ID
		})

		models = append(
			array.Map(
				array.UniqBy(
					array.Filter(
						array.Sort(rooms, func(item1 repo.Room, item2 repo.Room) bool {
							return item1.Id > item2.Id
						}),
						func(item repo.Room, _ int) bool {
							if strings.TrimSpace(item.SystemPrompt) == "" {
								return false
							}

							return true
						},
					),
					func(item repo.Room) string {
						return item.Model + strings.TrimSpace(item.SystemPrompt)
					},
				),
				func(item repo.Room, _ int) controllers.Model {
					model := modelIDMap[item.Model]
					avatarUrl := item.AvatarUrl
					if avatarUrl == "" {
						avatarUrl = model.AvatarURL
					}

					description := item.Description
					if description == "" {
						description = item.SystemPrompt
					}

					return controllers.Model{
						ID:            fmt.Sprintf("v2@%s|%d", service.HomeModelTypeRooms, item.Id),
						Name:          item.Name,
						AvatarURL:     avatarUrl,
						Description:   description,
						Category:      "数字人",
						IsImage:       model.IsImage,
						SupportVision: model.SupportVision,
						VersionMin:    model.VersionMin,
						VersionMax:    model.VersionMax,
						Tag:           model.Tag,
						TagTextColor:  model.TagTextColor,
						TagBgColor:    model.TagBgColor,
						IsNew:         model.IsNew,
						IsChat:        model.IsChat,
						PriceInfo:     model.PriceInfo,
					}
				},
			),
			models...,
		)
	}

	return webCtx.JSON(models)
}

// loadRawModels Load all large language models
func (ctl *ModelController) loadRawModels(ctx context.Context, client *auth.ClientInfo, user *auth.UserOptional) []controllers.Model {
	models := array.Map(ctl.svc.Chat.Models(ctx, true), func(item repo.Model, _ int) controllers.Model {
		priceInfo := ctl.generatePriceInfo(item)
		ret := controllers.Model{
			ID:            item.ModelId,
			Name:          item.Name,
			ShortName:     item.ShortName,
			Description:   item.Description,
			PriceInfo:     priceInfo,
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
			IsDefault:     item.ModelId == "gpt-4o-mini",
		}

		if misc.VersionOlder(client.Version, "2.0.0") {
			if item.Meta.InputPrice == 0 && item.Meta.OutputPrice == 0 && ret.Tag != "限免" {
				ret.Tag = "限免"
				ret.TagTextColor = "#ffffff"
				ret.TagBgColor = "#5694ED"
			}
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

	sortPriority := []string{"OpenAI", "Anthropic", "Google", "xAI", "Amazon", "Meta", "百度", "阿里", "科大讯飞"}
	models = array.Sort(models, func(i, j controllers.Model) bool {
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

	return models
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

	models := ctl.svc.Chat.Models(ctx, true)
	modelIDMap := array.ToMap(models, func(item repo.Model, _ int) string {
		return item.ModelId
	})

	// 类型：model
	homeModels = append(
		homeModels,
		array.Map(
			array.Filter(models, func(item repo.Model, _ int) bool { return item.Status == repo.ModelStatusEnabled }),
			func(item repo.Model, _ int) service.HomeModel {
				return service.HomeModel{
					Type:          service.HomeModelTypeModel,
					ID:            item.ModelId,
					Name:          item.Name,
					ModelID:       item.ModelId,
					ModelName:     item.Name,
					AvatarURL:     item.AvatarUrl,
					SupportVision: item.Meta.Vision,
				}
			},
		)...,
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
				ModelID:       model.ModelId,
				ModelName:     model.Name,
				SupportVision: model.Meta.Vision,
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
				avatarUrl := item.AvatarUrl
				if avatarUrl == "" {
					avatarUrl = model.AvatarUrl
				}

				return service.HomeModel{
					Type:          service.HomeModelTypeRooms,
					ID:            strconv.Itoa(int(item.Id)),
					Name:          item.Name,
					AvatarURL:     avatarUrl,
					ModelID:       model.ModelId,
					ModelName:     model.Name,
					SupportVision: model.Meta.Vision,
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

	modelArr := ctl.svc.Chat.Models(ctx, true)
	models := array.ToMap(modelArr, func(item repo.Model, _ int) string {
		return item.ModelId
	})
	homeModel, err := ctl.userSrv.QueryHomeModel(ctx, models, userID, key)
	if err != nil {
		key = "model|" + modelArr[0].ModelId
		homeModel, err = ctl.userSrv.QueryHomeModel(ctx, models, userID, key)
		if err != nil {
			return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
		}
	}

	return webCtx.JSON(web.M{
		"data": homeModel,
	})
}

type ModelPriceInfo struct {
	Input  int    `json:"input,omitempty"`
	Output int    `json:"output,omitempty"`
	Note   string `json:"note,omitempty"`
}

func (ctl *ModelController) generatePriceInfo(item repo.Model) string {
	data, _ := json.Marshal(ModelPriceInfo{
		Input:  item.Meta.InputPrice,
		Output: item.Meta.OutputPrice,
		Note: ternary.If(
			item.Meta.OutputPrice > 0 || item.Meta.InputPrice > 0,
			"每消耗 1000 Token 将扣除对应数量的智慧果，单次问答若不足 1 智慧果，按 1 智慧果计费。",
			"",
		),
	})

	return string(data)
}
