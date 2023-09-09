package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fvbommel/sortorder"
	"github.com/mylxsw/go-utils/ternary"

	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/internal/service"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"github.com/mylxsw/aidea-server/internal/youdao"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/str"
)

const (
	AllInOneIslandID            = "all-in-one"
	DefaultImageCompletionModel = "sb-stable-diffusion-xl-1024-v1-0"
)

// CreativeIslandController 创作岛
type CreativeIslandController struct {
	conf         *config.Config
	quotaRepo    *repo.QuotaRepo          `autowire:"@"`
	queue        *queue.Queue             `autowire:"@"`
	translater   youdao.Translater        `autowire:"@"`
	creativeRepo *repo.CreativeRepo       `autowire:"@"`
	securitySrv  *service.SecurityService `autowire:"@"`
}

// NewCreativeIslandController create a new CreativeIslandController
func NewCreativeIslandController(resolver infra.Resolver, conf *config.Config) web.Controller {
	ctl := CreativeIslandController{conf: conf}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *CreativeIslandController) Register(router web.Router) {
	router.Group("/creative", func(router web.Router) {
		router.Get("/items", ctl.Items)
	})

	router.Group("/creative-island", func(router web.Router) {
		router.Get("/histories", ctl.Histories)
		router.Get("/histories/{hid}", ctl.HistoryItem)
		router.Delete("/histories/{hid}", ctl.DeleteHistoryItem)

		router.Post("/histories/{hid}/share", ctl.ShareHistoryItem)
		router.Delete("/histories/{hid}/share", ctl.CancelShareHistoryItem)

		router.Get("/capacity", ctl.Capacity)
		router.Get("/models", ctl.Models)
		router.Get("/filters", ctl.ImageStyles)

		// 文生图、图生图
		router.Post("/completions", ctl.Completions)
		router.Post("/completions/evaluate", ctl.CompletionsEvaluate)

		// 图片放大
		router.Post("/completions/upscale", ctl.ImageUpscale)
		// 图片上色
		router.Post("/completions/colorize", ctl.ImageColorize)
	})
}

type CreativeIslandItem struct {
	ID           string `json:"id,omitempty"`
	Title        string `json:"title,omitempty"`
	TitleColor   string `json:"title_color,omitempty"`
	PreviewImage string `json:"preview_image,omitempty"`
	RouteURI     string `json:"route_uri,omitempty"`
}

func (ctl *CreativeIslandController) Items(ctx context.Context, webCtx web.Context, client *auth.ClientInfo) web.Response {
	items := []CreativeIslandItem{
		{
			ID:           "text-to-image",
			Title:        "文生图",
			TitleColor:   "FFFFFFFF",
			PreviewImage: "https://ssl.aicode.cc/ai-server/assets/background/image-text-to-image.jpeg-thumb1000",
			RouteURI:     "/creative-draw/create?mode=text-to-image&id=text-to-image",
		},
		{
			ID:           "image-to-image",
			Title:        "图生图",
			TitleColor:   "FFFFFFFF",
			PreviewImage: "https://ssl.aicode.cc/ai-server/assets/background/image-image-to-image.jpeg-thumb1000",
			RouteURI:     "/creative-draw/create?mode=image-to-image&id=image-to-image",
		},
	}

	if client != nil && helper.VersionNewer(client.Version, "1.0.2") && ctl.conf.EnableDeepAI {
		items = append(items, CreativeIslandItem{
			ID:           "image-upscale",
			Title:        "超分辨率",
			TitleColor:   "FFFFFFFF",
			PreviewImage: "https://ssl.aicode.cc/ai-server/assets/background/super-res.jpeg-thumb1000",
			RouteURI:     "/creative-draw/create-upscale",
		})

		items = append(items, CreativeIslandItem{
			ID:           "image-colorize",
			Title:        "图片上色",
			TitleColor:   "FFFFFFFF",
			PreviewImage: "https://ssl.aicode.cc/ai-server/assets/background/image-colorizev2.jpeg-thumb1000",
			RouteURI:     "/creative-draw/create-colorize",
		})
	}

	return webCtx.JSON(web.M{
		"data": items,
	})
}

type CreativeIslandCapacity struct {
	ShowAIRewrite            bool          `json:"show_ai_rewrite,omitempty"`
	ShowUpscaleBy            bool          `json:"show_upscale_by,omitempty"`
	ShowNegativeText         bool          `json:"show_negative_text,omitempty"`
	ShowStyle                bool          `json:"show_style,omitempty"`
	ShowImageCount           bool          `json:"show_image_count,omitempty"`
	ShowSeed                 bool          `json:"show_seed,omitempty"`
	ShowPromptForImage2Image bool          `json:"show_prompt_for_image2image,omitempty"`
	AllowRatios              []string      `json:"allow_ratios,omitempty"`
	VendorModels             []VendorModel `json:"vendor_models,omitempty"`
	Filters                  []ImageStyle  `json:"filters,omitempty"`
	AllowUpscaleBy           []string      `json:"allow_upscale_by,omitempty"`
	ShowImageStrength        bool          `json:"show_image_strength,omitempty"`
}

// Models 可用的模型列表
func (ctl *CreativeIslandController) Models(ctx context.Context, webCtx web.Context) web.Response {
	return webCtx.JSON(web.M{
		"data": ctl.loadAllModels(ctx),
	})
}

// loadAllModels 加载所有的模型
// TODO 加缓存
func (ctl *CreativeIslandController) loadAllModels(ctx context.Context) []repo.ImageModel {
	models, err := ctl.creativeRepo.Models(ctx)
	if err != nil {
		log.Errorf("get models failed: %v", err)
	}

	return array.Filter(models, func(m repo.ImageModel, _ int) bool {
		if m.Vendor == "leapai" {
			return ctl.conf.EnableLeapAI
		}

		if m.Vendor == "stabilityai" {
			return ctl.conf.EnableStabilityAI
		}

		if m.Vendor == "fromston" {
			return ctl.conf.EnableFromstonAI
		}

		if m.Vendor == "getimgai" {
			return ctl.conf.EnableGetimgAI
		}

		return true
	})
}

// ImageStyles 图片风格，历史遗留问题可能部分代码也是用了 Filter 这个名字
// TODO 加缓存
func (ctl *CreativeIslandController) ImageStyles(ctx context.Context, webCtx web.Context) web.Response {
	filters, err := ctl.creativeRepo.Filters(ctx)
	if err != nil {
		log.Errorf("get filters failed: %v", err)
		return webCtx.JSONError(common.ErrInternalError, http.StatusInternalServerError)
	}

	// 查询所有可用的模型，转换为 map[模型ID]模型ID
	availableModels := array.ToMap(
		array.Map(ctl.loadAllModels(ctx), func(item repo.ImageModel, _ int) string {
			return item.ModelId
		}),
		func(val string, _ int) string {
			return val
		},
	)

	// 过滤掉当前没有启用的模型
	filters = array.Filter(filters, func(item repo.ImageFilter, _ int) bool {
		_, ok := availableModels[item.ModelId]
		return ok
	})

	return webCtx.JSON(web.M{
		"data": filters,
	})
}

// Capacity 文生图、图生图支持的能力，用于控制客户端显示哪些允许用户配置的参数
func (ctl *CreativeIslandController) Capacity(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	mode := webCtx.InputWithDefault("mode", "text-to-image")
	id := webCtx.Input("id")

	log.WithFields(log.Fields{"id": id, "mode": mode}).Debugf("creative capacity request")

	filters := array.Sort(
		array.Filter(ctl.getAllImageStyles(ctx), func(item ImageStyle, index int) bool {
			return str.In(mode, item.Supports)
		}),
		func(f1, f2 ImageStyle) bool { return sortorder.NaturalLess(f1.Name, f2.Name) },
	)

	var models []VendorModel
	if user.InternalUser() && user.WithLab {
		models = array.Sort(array.Filter(ctl.getAllModels(ctx), func(v VendorModel, _ int) bool { return v.Enabled }), func(v1, v2 VendorModel) bool {
			return sortorder.NaturalLess(v1.Name, v2.Name)
		})

		models = array.Map(models, func(item VendorModel, _ int) VendorModel {
			if !user.InternalUser() || !user.WithLab {
				item.Vendor = ""
			}

			return item
		})
	}

	return webCtx.JSON(CreativeIslandCapacity{
		ShowAIRewrite:            true,
		ShowUpscaleBy:            true,
		AllowRatios:              []string{"1:1" /*"4:3", "3:4",*/, "3:2", "2:3" /*"16:9"*/},
		ShowStyle:                true,
		ShowNegativeText:         true,
		ShowSeed:                 user.InternalUser() && user.WithLab,
		ShowImageCount:           user.InternalUser() && user.WithLab,
		ShowPromptForImage2Image: true,
		Filters:                  filters,
		VendorModels:             models,
		AllowUpscaleBy:           []string{"x1", "x2", "x4"},
		ShowImageStrength:        true,
	})
}

// ShareHistoryItem 分享创作到发现页
func (ctl *CreativeIslandController) ShareHistoryItem(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	hid, _ := strconv.Atoi(webCtx.PathVar("hid"))
	if hid <= 0 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	err := ctl.creativeRepo.ShareCreativeHistoryToGallery(ctx, user.ID, user.Name, int64(hid))
	if err != nil {
		if err == repo.ErrNotFound {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrNotFound), http.StatusNotFound)
		}

		log.WithFields(log.Fields{
			"uid":    user.ID,
			"his_id": hid,
		}).Errorf("share creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// CancelShareHistoryItem 取消分享创作到发现页
func (ctl *CreativeIslandController) CancelShareHistoryItem(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	hid, _ := strconv.Atoi(webCtx.PathVar("hid"))
	if hid <= 0 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	err := ctl.creativeRepo.CancelCreativeHistoryShare(ctx, user.ID, int64(hid))
	if err != nil {
		log.WithFields(log.Fields{
			"uid":    user.ID,
			"his_id": hid,
		}).Errorf("cancel share creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// Histories 获取创作岛项目的历史记录
func (ctl *CreativeIslandController) Histories(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	page := webCtx.Int64Input("page", 1)
	if page < 1 || page > 1000 {
		page = 1
	}

	perPage := webCtx.Int64Input("per_page", 20)
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	items, meta, err := ctl.creativeRepo.HistoryRecordPaginate(ctx, user.ID, repo.CreativeHistoryQuery{
		Page:        page,
		PerPage:     perPage,
		IslandId:    AllInOneIslandID,
		IslandModel: webCtx.Input("model"),
	})
	if err != nil {
		log.Errorf("query creative items failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	// 以下字段不需要返回给前端
	items = array.Map(items, func(item repo.CreativeHistoryItem, _ int) repo.CreativeHistoryItem {
		//  Arguments 只保留必须的 image 字段，用于客户端区分是文生图还是图生图
		var arguments map[string]any
		_ = json.Unmarshal([]byte(item.Arguments), &arguments)

		item.Arguments = ""
		if arguments != nil {
			image, ok := arguments["image"]
			if ok {
				data, _ := json.Marshal(map[string]any{"image": image})
				item.Arguments = string(data)
			}
		}

		item.Prompt = ""
		item.QuotaUsed = 0

		switch item.IslandType {
		case int64(repo.IslandTypeImage):
			if arguments != nil {
				if _, ok := arguments["image"]; ok {
					item.IslandTitle = "图生图"
				}
			}

			if item.IslandTitle == "" {
				item.IslandTitle = "文生图"
			}
		case int64(repo.IslandTypeUpscale):
			item.IslandTitle = "超分辨率"
		case int64(repo.IslandTypeImageColorization):
			item.IslandTitle = "图片上色"
		}

		return item
	})

	// TODO 正式发布后，不返回 ImageStyles，这里只是发布前预览
	filters := ctl.getAllImageStyles(ctx)
	filters = array.Map(filters, func(filter ImageStyle, _ int) ImageStyle {
		filter.PreviewImage = ""
		return filter
	})

	return webCtx.JSON(web.M{
		"data":      items,
		"filters":   filters,
		"page":      meta.Page,
		"per_page":  meta.PerPage,
		"total":     meta.Total,
		"last_page": meta.LastPage,
	})
}

type CreativeHistoryItemResp struct {
	repo.CreativeHistoryItem
	ShowBetaFeature bool `json:"show_beta_feature,omitempty"`
}

// HistoryItem 获取创作岛项目的历史记录详情
func (ctl *CreativeIslandController) HistoryItem(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	hid, _ := strconv.Atoi(webCtx.PathVar("hid"))
	if hid <= 0 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	item, err := ctl.creativeRepo.FindHistoryRecord(ctx, user.ID, int64(hid))
	if err != nil {
		if err == repo.ErrNotFound {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrNotFound), http.StatusNotFound)
		}

		log.Errorf("query creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(CreativeHistoryItemResp{
		CreativeHistoryItem: *item,
		ShowBetaFeature:     user.InternalUser() && user.WithLab,
	})
}

// DeleteHistoryItem 删除创作岛项目的历史记录
func (ctl *CreativeIslandController) DeleteHistoryItem(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	hid, _ := strconv.Atoi(webCtx.PathVar("hid"))
	if hid <= 0 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	log.WithFields(log.Fields{
		"uid":    user.ID,
		"his_id": hid,
	}).Infof("delete creative item")

	if err := ctl.creativeRepo.DeleteHistoryRecord(ctx, user.ID, int64(hid)); err != nil {
		log.Errorf("delete creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// CompletionsEvaluate 创作岛项目文本生成 价格评估
func (ctl *CreativeIslandController) CompletionsEvaluate(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	req, errResp := ctl.resolveImageCompletionRequest(ctx, webCtx, user)
	if errResp != nil {
		return errResp
	}

	// 检查用户是否有足够的智慧果
	quota, err := ctl.quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if !user.InternalUser() {
		req.Quota = 0
	}

	return webCtx.JSON(web.M{"cost": req.Quota, "enough": quota.Quota >= quota.Used+req.Quota, "wait_duration": 45})
}

// resolveImageCompletionRequest 解析创作岛项目图片生成请求参数
func (ctl *CreativeIslandController) resolveImageCompletionRequest(ctx context.Context, webCtx web.Context, user *auth.User) (*queue.ImageCompletionPayload, web.Response) {
	image := webCtx.Input("image")
	if image != "" && !str.HasPrefixes(image, []string{"http://", "https://"}) {
		return nil, webCtx.JSONError("invalid image", http.StatusBadRequest)
	}

	promptTags := array.Uniq(array.Filter(
		strings.Split(webCtx.Input("prompt_tags"), ","),
		func(tag string, _ int) bool {
			return tag != ""
		},
	))

	prompt := strings.Trim(strings.ReplaceAll(strings.TrimSpace(webCtx.Input("prompt")), "，", ","), ",")
	if prompt == "" && image == "" {
		return nil, webCtx.JSONError("prompt is required", http.StatusBadRequest)
	}

	negativePrompt := strings.ReplaceAll(strings.TrimSpace(webCtx.Input("negative_prompt")), "，", ",")
	if helper.WordCount(negativePrompt) > 1000 {
		return nil, webCtx.JSONError(fmt.Sprintf("排除内容输入字数不能超过 %d", 1000), http.StatusBadRequest)
	}

	imageCount := webCtx.Int64Input("image_count", 1)
	if imageCount < 1 || imageCount > 4 {
		return nil, webCtx.JSONError("invalid image count", http.StatusBadRequest)
	}

	steps := webCtx.IntInput("steps", 50)
	if !array.In(steps, []int{30, 50, 100, 150}) {
		return nil, webCtx.JSONError("invalid steps", http.StatusBadRequest)
	}

	// AI 自动改写
	aiRewrite := webCtx.InputWithDefault("ai_rewrite", "false") == "true"
	// 图生图模式，不启用 AI 改写
	if image != "" {
		aiRewrite = false
	}

	mode := webCtx.InputWithDefault("mode", "canny")
	if !array.In(mode, []string{"canny", "mlsd", "pose", "scribble"}) {
		mode = "canny"
	}

	upscaleBy := webCtx.InputWithDefault("upscale_by", "x1")
	if !array.In(upscaleBy, []string{"x1", "x2", "x4"}) {
		return nil, webCtx.JSONError("invalid upscale_by", http.StatusBadRequest)
	}

	stylePreset := webCtx.Input("style_preset")

	modelID := webCtx.InputWithDefault("model", DefaultImageCompletionModel)
	filterID := webCtx.Int64Input("filter_id", 0)
	var filterName string
	if filterID > 0 {
		filter := ctl.getStyleByID(ctx, filterID)
		if filter == nil {
			return nil, webCtx.JSONError("invalid filter_id", http.StatusBadRequest)
		}

		modelID = filter.ModelID
		filterName = filter.Name
	} else {
		// 如果没有指定 filter， 则自动根据模型补充 filter 信息
		mode := ternary.If(image != "", "image-to-image", "text-to-image")
		filter := ctl.getStyleByModelID(ctx, modelID, mode)
		if filter != nil {
			filterID = filter.ID
			filterName = filter.Name
		}
	}

	vendorModel := ctl.getVendorModel(ctx, modelID)
	if vendorModel == nil {
		return nil, webCtx.JSONError("invalid model", http.StatusBadRequest)
	}

	imageRatio := webCtx.InputWithDefault("image_ratio", "1:1")
	if !array.In(imageRatio, []string{"1:1", "4:3", "3:4", "3:2", "2:3", "16:9"}) {
		return nil, webCtx.JSONError("invalid image ratio", http.StatusBadRequest)
	}

	// 根据模型配置，自动调整相关参数（width/height）
	dimension := vendorModel.GetDimension(imageRatio)

	width, height := webCtx.IntInput("width", dimension.Width), webCtx.IntInput("height", dimension.Height)
	if width < 1 || height < 1 || width > 2048 || height > 2048 {
		return nil, webCtx.JSONError("invalid width or height", http.StatusBadRequest)
	}

	imageStrength := webCtx.Float64Input("image_strength", 0.5)
	if imageStrength < 0 || imageStrength > 1 {
		return nil, webCtx.JSONError("invalid image_strength", http.StatusBadRequest)
	}

	if imageStrength == 0 {
		imageStrength = 0.5
	}

	seed := webCtx.Int64Input("seed", int64(rand.Intn(2147483647)))
	if seed < 0 || seed > 2147483647 {
		return nil, webCtx.JSONError("invalid seed", http.StatusBadRequest)
	}

	return &queue.ImageCompletionPayload{
		Prompt:         prompt,
		NegativePrompt: negativePrompt,
		PromptTags:     promptTags,
		ImageCount:     imageCount,
		ImageRatio:     imageRatio,
		Width:          int64(width),
		Height:         int64(height),
		Steps:          int64(steps),
		Image:          image,
		AIRewrite:      aiRewrite,
		Mode:           mode,
		UpscaleBy:      upscaleBy,
		StylePreset:    stylePreset,
		Seed:           seed,
		ImageStrength:  1.0 - imageStrength,
		FilterID:       filterID,
		FilterName:     filterName,
		GalleryCopyID:  webCtx.Int64Input("gallery_copy_id", 0),

		UID:       user.ID,
		Quota:     int64(coins.GetUnifiedImageGenCoins()) * imageCount,
		CreatedAt: time.Now(),

		Vendor:    vendorModel.Vendor,
		Model:     vendorModel.Model,
		ModelName: vendorModel.Name,
	}, nil
}

func (ctl *CreativeIslandController) getAllModels(ctx context.Context) []VendorModel {
	return array.Map(ctl.loadAllModels(ctx), func(m repo.ImageModel, _ int) VendorModel {
		return VendorModel{
			ID:                m.ModelId,
			Name:              m.ModelName,
			Vendor:            m.Vendor,
			Model:             m.RealModel,
			Enabled:           m.Status == 1,
			Upscale:           m.ImageMeta.Upscale,
			ShowStyle:         m.ImageMeta.ShowStyle,
			ShowImageStrength: m.ImageMeta.ShowImageStrength,
			IntroURL:          m.ImageMeta.IntroURL,
			RatioDimensions:   m.ImageMeta.RatioDimensions,
		}
	})
}

func (ctl *CreativeIslandController) getVendorModel(ctx context.Context, modelID string) *VendorModel {
	models := ctl.getAllModels(ctx)
	for _, m := range models {
		if m.ID == modelID {
			return &m
		}
	}

	return nil
}

type ImageStyle struct {
	ID             int64    `json:"id,omitempty"`
	Name           string   `json:"name,omitempty"`
	PreviewImage   string   `json:"preview_image,omitempty"`
	Description    string   `json:"description,omitempty"`
	ModelID        string   `json:"-"`
	Prompt         string   `json:"-"`
	NegativePrompt string   `json:"-"`
	Supports       []string `json:"-"`
}

func (ctl *CreativeIslandController) getAllImageStyles(ctx context.Context) []ImageStyle {
	filters, err := ctl.creativeRepo.Filters(ctx)
	if err != nil {
		log.Errorf("get filters failed: %v", err)
		return []ImageStyle{}
	}

	return array.Map(filters, func(f repo.ImageFilter, _ int) ImageStyle {
		return ImageStyle{
			ID:             f.Id,
			Name:           f.Name,
			PreviewImage:   f.PreviewImage,
			Description:    f.Description,
			ModelID:        f.ModelId,
			Prompt:         f.ImageMeta.Prompt,
			NegativePrompt: f.ImageMeta.NegativePrompt,
			Supports:       f.ImageMeta.Supports,
		}
	})
}

func (ctl *CreativeIslandController) getStyleByID(ctx context.Context, styleID int64) *ImageStyle {
	filters := ctl.getAllImageStyles(ctx)
	if len(filters) == 0 {
		return nil
	}

	for _, f := range filters {
		if f.ID == styleID {
			return &f
		}
	}

	return nil
}

func (ctl *CreativeIslandController) getStyleByModelID(ctx context.Context, modelID string, mode string) *ImageStyle {
	filters := ctl.getAllImageStyles(ctx)
	if len(filters) == 0 {
		return nil
	}

	if len(filters) == 1 {
		return &filters[0]
	}

	matched := array.Filter(filters, func(item ImageStyle, _ int) bool {
		return item.ModelID == modelID && array.In(mode, item.Supports)
	})

	if len(matched) == 1 {
		return &matched[0]
	}

	return nil
}

type VendorModel struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	Vendor            string                    `json:"vendor,omitempty"`
	Model             string                    `json:"-"`
	Enabled           bool                      `json:"-"`
	Upscale           bool                      `json:"upscale,omitempty"`
	ShowStyle         bool                      `json:"show_style,omitempty"`
	ShowImageStrength bool                      `json:"show_image_strength,omitempty"`
	IntroURL          string                    `json:"intro_url,omitempty"`
	RatioDimensions   map[string]repo.Dimension `json:"–"`
}

func (vm VendorModel) defaultDimension(ratio string) repo.Dimension {
	switch ratio {
	case "1:1":
		return repo.Dimension{512, 512}
	case "4:3":
		return repo.Dimension{768, 576}
	case "3:4":
		return repo.Dimension{576, 768}
	case "3:2":
		return repo.Dimension{768, 512}
	case "2:3":
		return repo.Dimension{512, 768}
	case "16:9":
		return repo.Dimension{1024, 576}
	}

	return repo.Dimension{512, 512}
}

func (vm VendorModel) GetDimension(ratio string) repo.Dimension {
	if vm.RatioDimensions == nil {
		vm.RatioDimensions = map[string]repo.Dimension{}
	}

	dimension, ok := vm.RatioDimensions[ratio]
	if !ok {
		return vm.defaultDimension(ratio)
	}

	if dimension.Width == 0 || dimension.Height == 0 {
		def := vm.defaultDimension(ratio)
		if dimension.Width <= 0 {
			dimension.Width = def.Width
		}

		if dimension.Height <= 0 {
			dimension.Height = def.Height
		}
	}

	return dimension
}

func (ctl *CreativeIslandController) ImageUpscale(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	image := webCtx.Input("image")
	if image != "" && !str.HasPrefixes(image, []string{"http://", "https://"}) {
		return webCtx.JSONError("invalid image", http.StatusBadRequest)
	}

	// 图片地址检查
	if !strings.HasPrefix(image, ctl.conf.StorageDomain) {
		return webCtx.JSONError("invalid image", http.StatusBadRequest)
	}

	// 检查用户是否有足够的智慧果
	quota, err := ctl.quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	quotaConsume := int64(coins.GetUnifiedImageGenCoins())
	if quota.Quota < quota.Used+quotaConsume {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	upscaleBy := "x4"

	req := queue.ImageUpscalePayload{
		UserID:    user.ID,
		Image:     image,
		UpscaleBy: upscaleBy,
		Quota:     quotaConsume,
		CreatedAt: time.Now(),
	}

	// 加入异步任务队列
	taskID, err := ctl.queue.Enqueue(&req, queue.NewImageUpscaleTask)
	if err != nil {
		log.Errorf("enqueue task failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}
	log.WithFields(log.Fields{"task_id": taskID}).Debugf("enqueue task success: %s", taskID)

	creativeItem := repo.CreativeItem{
		IslandId:   AllInOneIslandID,
		IslandType: repo.IslandTypeUpscale,
		TaskId:     taskID,
		Status:     repo.CreativeStatusPending,
	}

	arg := repo.CreativeRecordArguments{
		Image:     image,
		UpscaleBy: upscaleBy,
	}

	// 保存历史记录
	if _, err := ctl.creativeRepo.CreateRecordWithArguments(ctx, user.ID, &creativeItem, &arg); err != nil {
		log.Errorf("create creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"task_id": taskID, // 任务 ID
		"wait":    60,     // 等待时间
	})
}

func (ctl *CreativeIslandController) ImageColorize(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	image := webCtx.Input("image")
	if image != "" && !str.HasPrefixes(image, []string{"http://", "https://"}) {
		return webCtx.JSONError("invalid image", http.StatusBadRequest)
	}

	// 图片地址检查
	if !strings.HasPrefix(image, ctl.conf.StorageDomain) {
		return webCtx.JSONError("invalid image", http.StatusBadRequest)
	}

	// 检查用户是否有足够的智慧果
	quota, err := ctl.quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	quotaConsume := int64(coins.GetUnifiedImageGenCoins())
	if quota.Quota < quota.Used+quotaConsume {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	req := queue.ImageColorizationPayload{
		UserID:    user.ID,
		Image:     image,
		Quota:     quotaConsume,
		CreatedAt: time.Now(),
	}

	// 加入异步任务队列
	taskID, err := ctl.queue.Enqueue(&req, queue.NewImageColorizationTask)
	if err != nil {
		log.Errorf("enqueue task failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}
	log.WithFields(log.Fields{"task_id": taskID}).Debugf("enqueue task success: %s", taskID)

	creativeItem := repo.CreativeItem{
		IslandId:   AllInOneIslandID,
		IslandType: repo.IslandTypeImageColorization,
		TaskId:     taskID,
		Status:     repo.CreativeStatusPending,
	}

	arg := repo.CreativeRecordArguments{
		Image: image,
	}

	// 保存历史记录
	if _, err := ctl.creativeRepo.CreateRecordWithArguments(ctx, user.ID, &creativeItem, &arg); err != nil {
		log.Errorf("create creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"task_id": taskID, // 任务 ID
		"wait":    60,     // 等待时间
	})
}

// Completions 创作岛项目文本生成
func (ctl *CreativeIslandController) Completions(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	req, errResp := ctl.resolveImageCompletionRequest(ctx, webCtx, user)
	if errResp != nil {
		return errResp
	}

	// 图片地址检查
	if req.Image != "" && !strings.HasPrefix(req.Image, "https://ssl.aicode.cc/") {
		return webCtx.JSONError("invalid image", http.StatusBadRequest)
	}

	// stabilityai 和 fromston 生成的图片为正方形
	if req.Image != "" && array.In(req.Vendor, []string{"fromston", "stabilityai"}) {
		req.Image = uploader.BuildImageURLWithFilter(req.Image, "fix_square_1024", ctl.conf.StorageDomain)
	}

	// 检查用户是否有足够的智慧果
	quota, err := ctl.quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if quota.Quota < quota.Used+req.Quota {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	// 内容安全检测
	if checkRes := ctl.securitySrv.PromptDetect(req.Prompt); checkRes != nil {
		if !checkRes.Safe {
			log.WithFields(log.Fields{
				"user_id": user.ID,
				"details": checkRes,
				"content": req.Prompt,
			}).Warningf("用户 %d 违规，违规内容：%s", user.ID, checkRes.Reason)
			return webCtx.JSONError("内容违规，已被系统拦截，如有疑问邮件联系：support@aicode.cc", http.StatusNotAcceptable)
		}
	}

	// 加入异步任务队列
	taskID, err := ctl.queue.Enqueue(req, queue.NewImageCompletionTask)
	if err != nil {
		log.Errorf("enqueue task failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}
	log.WithFields(log.Fields{"task_id": taskID}).Debugf("enqueue task success: %s", taskID)

	// 保存历史记录
	creativeItem, arg := ctl.buildHistorySaveRecord(req, taskID)
	if _, err := ctl.creativeRepo.CreateRecordWithArguments(ctx, user.ID, &creativeItem, &arg); err != nil {
		log.Errorf("create creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"task_id": taskID, // 任务 ID
		"wait":    60,     // 等待时间
	})
}

// buildHistorySaveRecord 构建保存历史记录的 CreativeItem
func (*CreativeIslandController) buildHistorySaveRecord(req *queue.ImageCompletionPayload, taskID string) (repo.CreativeItem, repo.CreativeRecordArguments) {
	creativeItem := repo.CreativeItem{
		IslandId:    AllInOneIslandID,
		IslandType:  repo.IslandTypeImage,
		IslandModel: req.Model,
		Prompt:      req.Prompt,
		TaskId:      taskID,
		Status:      repo.CreativeStatusPending,
	}
	return creativeItem, repo.CreativeRecordArguments{
		NegativePrompt: req.NegativePrompt,
		PromptTags:     req.PromptTags,
		Width:          req.Width,
		Height:         req.Height,
		Steps:          req.Steps,
		ImageCount:     req.ImageCount,
		ImageRatio:     req.ImageRatio,
		StylePreset:    req.StylePreset,
		Mode:           req.Mode,
		Image:          req.Image,
		UpscaleBy:      req.UpscaleBy,
		AIRewrite:      req.AIRewrite,
		ModelID:        req.GetModel(),
		ModelName:      req.ModelName,
		FilterID:       req.FilterID,
		FilterName:     req.FilterName,
		GalleryCopyID:  req.GalleryCopyID,
		Seed:           req.Seed,
	}
}
