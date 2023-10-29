package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	openaiHelper "github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/internal/service"
	"github.com/mylxsw/aidea-server/internal/youdao"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/ternary"
	"github.com/sashabaranov/go-openai"
)

// CreativeIslandController 创作岛
type CreativeIslandController struct {
	conf         *config.Config
	quotaRepo    *repo.QuotaRepo          `autowire:"@"`
	queue        *queue.Queue             `autowire:"@"`
	queueRepo    *repo.QueueRepo          `autowire:"@"`
	trans        youdao.Translater        `autowire:"@"`
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
	router.Group("/creative-island", func(router web.Router) {
		router.Get("/items", ctl.List)
		router.Get("/items/{id}", ctl.Item)
		router.Get("/items/{id}/tasks", ctl.completionsTasks)

		router.Get("/histories", ctl.histories)
		router.Get("/items/{id}/histories", ctl.itemHistories)
		router.Get("/items/{id}/histories/{hid}", ctl.historyItem)
		router.Delete("/items/{id}/histories/{hid}", ctl.deleteHistoryItem)

		router.Post("/completions/{id}", ctl.completions)
		router.Post("/completions/{id}/evaluate", ctl.completionsEvaluate)

		router.Get("/gallery", ctl.gallery)
	})
}

// gallery 创作岛项目的图库
func (ctl *CreativeIslandController) gallery(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	mode := webCtx.InputWithDefault("mode", "default")

	userId := user.ID
	limit := int64(100)
	if mode == "all" && user.InternalUser() {
		userId = 0
		limit = 500
	}

	model := webCtx.Input("model")
	items, err := ctl.creativeRepo.UserGallery(ctx, userId, model, limit)
	if err != nil {
		log.Errorf("query creative items failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data": array.Map(items, func(item repo.CreativeHistoryItem, _ int) repo.CreativeHistoryItem {
			if item.UserID != user.ID && userId != 0 {
				// 客户端处理：如果用户ID为0，则该项目不可点击
				item.UserID = 0
			}

			return item
		}),
	})
}

// histories 获取创作岛项目的历史记录
func (ctl *CreativeIslandController) histories(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	page := webCtx.Int64Input("page", 1)
	if page < 1 || page > 1000 {
		page = 1
	}

	perPage := webCtx.Int64Input("per_page", 20)
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	mode := webCtx.Input("mode")
	if mode != "" && !array.In(mode, []string{"creative-island", "image-draw"}) {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	items, meta, err := ctl.creativeRepo.HistoryRecordPaginate(ctx, user.ID, repo.CreativeHistoryQuery{
		Page:    page,
		PerPage: perPage,
		Mode:    mode,
	})
	if err != nil {
		log.Errorf("query creative items failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data": array.Map(items, func(item repo.CreativeHistoryItem, _ int) repo.CreativeHistoryItem {
			// 客户端目前不支持封禁状态展示，这里转换为失败
			if item.Status == int64(repo.CreativeStatusForbid) {
				item.Status = int64(repo.CreativeStatusFailed)
			}

			return item
		}),
		"page":      meta.Page,
		"per_page":  meta.PerPage,
		"total":     meta.Total,
		"last_page": meta.LastPage,
	})
}

// itemHistories 获取创作岛项目的历史记录
func (ctl *CreativeIslandController) itemHistories(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	id := webCtx.PathVar("id")
	if id == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInvalidRequest), http.StatusBadRequest)
	}
	items, _, err := ctl.creativeRepo.HistoryRecordPaginate(ctx, user.ID, repo.CreativeHistoryQuery{
		IslandId: id,
		Page:     1,
		PerPage:  100,
	})
	if err != nil {
		log.Errorf("query creative items failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data": array.Map(items, func(item repo.CreativeHistoryItem, _ int) repo.CreativeHistoryItem {
			// 客户端目前不支持封禁状态展示，这里转换为失败
			if item.Status == int64(repo.CreativeStatusForbid) {
				item.Status = int64(repo.CreativeStatusFailed)
			}

			return item
		}),
	})
}

// historyItem 获取创作岛项目的历史记录详情
func (ctl *CreativeIslandController) historyItem(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	id := webCtx.PathVar("id")
	if id == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	hid, _ := strconv.Atoi(webCtx.PathVar("hid"))
	if hid <= 0 {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	item, err := ctl.creativeRepo.FindHistoryRecord(ctx, user.ID, int64(hid))
	if err != nil {
		if err == repo.ErrNotFound {
			return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrNotFound), http.StatusNotFound)
		}

		log.Errorf("query creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	// 客户端目前不支持封禁状态展示，这里转换为失败
	if item.Status == int64(repo.CreativeStatusForbid) {
		item.Status = int64(repo.CreativeStatusFailed)
	}

	return webCtx.JSON(item)
}

// deleteHistoryItem 删除创作岛项目的历史记录
func (ctl *CreativeIslandController) deleteHistoryItem(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	id := webCtx.PathVar("id")
	if id == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	hid, _ := strconv.Atoi(webCtx.PathVar("hid"))
	if hid <= 0 {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	log.WithFields(log.Fields{
		"uid":    user.ID,
		"id":     id,
		"his_id": hid,
	}).Infof("delete creative item")

	if err := ctl.creativeRepo.DeleteHistoryRecord(ctx, user.ID, int64(hid)); err != nil {
		log.Errorf("delete creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// List 列出创作岛的所有项目
func (ctl *CreativeIslandController) List(ctx context.Context, webCtx web.Context, client *auth.ClientInfo) web.Response {
	mode := webCtx.Input("mode")
	if mode != "" && !array.In(mode, []string{"creative-island", "image-draw"}) {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	islands, err := ctl.creativeRepo.Islands(ctx)
	if err != nil {
		log.Errorf("query creative islands failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	islands = array.Filter(islands, func(item repo.CreativeIsland, _ int) bool {
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

	var items []CreativeIslandItem
	var categories []string
	var backgroundImage string

	switch mode {
	case "creative-island":
		categories = []string{"热门", "创作", "生活", "职场", "娱乐"}
		islands = array.Filter(islands, func(item repo.CreativeIsland, _ int) bool {
			return !array.In(CreativeIslandModelType(item.ModelType), imageTypes)
		})
		items = array.Map(islands, func(item repo.CreativeIsland, _ int) CreativeIslandItem {
			return CreativeIslandItemFromModel(item)
		})
	case "image-draw":
		categories = []string{"图生图", "文生图"}
		backgroundImage = "https://img.freepik.com/free-vector/modern-colorful-soft-watercolor-texture-background_1035-22725.jpg"
		islands = array.Filter(islands, func(item repo.CreativeIsland, _ int) bool {
			return array.In(CreativeIslandModelType(item.ModelType), imageTypes)
		})
		items = array.Map(islands, func(item repo.CreativeIsland, _ int) CreativeIslandItem {
			// 不能暴漏给客户端的字段
			item.Extension.AIPrompt = ""
			return CreativeIslandItemFromModel(item)
		})
	default:
		categories = []string{"热门", "绘图", "创作", "生活", "职场", "娱乐"}
		items = array.Map(islands, func(item repo.CreativeIsland, _ int) CreativeIslandItem {
			// 不能暴漏给客户端的字段
			item.Extension.AIPrompt = ""
			return CreativeIslandItemFromModel(item)
		})
	}

	return webCtx.JSON(web.M{"items": items, "categories": categories, "background_image": backgroundImage})

}

var imageTypes = []CreativeIslandModelType{
	CreativeIslandModelTypeImage,
	CreativeIslandModelTypeImageToImage,
}

// Item 获取创作岛项目详情
func (ctl *CreativeIslandController) Item(ctx context.Context, webCtx web.Context) web.Response {
	id := webCtx.PathVar("id")
	island, err := ctl.creativeRepo.Island(ctx, id)
	if err != nil {
		if err == repo.ErrNotFound {
			return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrNotFound), http.StatusNotFound)
		}

		log.Errorf("query creative island failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	// 不能暴漏给客户端的字段
	island.Extension.AIPrompt = ""

	return webCtx.JSON(CreativeIslandItemFromModel(*island))
}

func (ctl *CreativeIslandController) completionsTasks(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	item := ctl.resolveIslandItem(webCtx.PathVar("id"))
	if item == nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrNotFound), http.StatusNotFound)
	}

	tasks, err := ctl.queueRepo.Tasks(ctx, user.ID, queue.ResolveTaskType(item.Vendor, item.Model))
	if err != nil {
		log.With(item).Errorf("query tasks failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data": tasks,
	})
}

func (ctl *CreativeIslandController) resolveIslandItem(id string) *CreativeIslandItem {
	island, err := ctl.creativeRepo.Island(context.Background(), id)
	if err != nil {
		return nil
	}

	ret := CreativeIslandItemFromModel(*island)
	return &ret
}

// completionsEvaluate 创作岛项目文本生成 价格评估
func (ctl *CreativeIslandController) completionsEvaluate(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	item := ctl.resolveIslandItem(webCtx.PathVar("id"))
	if item == nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrNotFound), http.StatusNotFound)
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if !user.InternalUser() {
		return webCtx.JSON(web.M{"cost": 0})
	}

	switch item.Vendor {
	case "openai":
		return ctl.completionsOpenAI(ctx, webCtx, item, user, true)
	case "deepai":
		return ctl.completionsDeepAI(ctx, webCtx, item, user, true)
	case "stabilityai":
		return ctl.completionsStabilityAI(ctx, webCtx, item, user, true)
	case "leapai":
		return ctl.completionsLeapAI(ctx, webCtx, item, user, true)
	default:
	}

	return webCtx.JSON(web.M{})
}

// completions 创作岛项目文本生成
func (ctl *CreativeIslandController) completions(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	item := ctl.resolveIslandItem(webCtx.PathVar("id"))
	if item == nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrNotFound), http.StatusNotFound)
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	switch item.Vendor {
	case "openai":
		return ctl.completionsOpenAI(ctx, webCtx, item, user, false)
	case "deepai":
		return ctl.completionsDeepAI(ctx, webCtx, item, user, false)
	case "stabilityai":
		return ctl.completionsStabilityAI(ctx, webCtx, item, user, false)
	case "leapai":
		return ctl.completionsLeapAI(ctx, webCtx, item, user, false)
	default:
	}

	return webCtx.JSON(web.M{})
}

// completionsDeepAI 创作岛项目文本生成 - DeepAI
func (ctl *CreativeIslandController) completionsDeepAI(ctx context.Context, webCtx web.Context, item *CreativeIslandItem, user *auth.User, evaluate bool) web.Response {
	prompt := strings.ReplaceAll(strings.TrimSpace(webCtx.Input("prompt")), "，", ",")
	if item.NoPrompt {
		prompt = item.Prompt
	} else {
		if prompt == "" {
			return webCtx.JSONError("prompt is required", http.StatusBadRequest)
		}
	}

	if helper.WordCount(prompt) > item.WordCount {
		return webCtx.JSONError(fmt.Sprintf("创作内容输入字数不能超过 %d", item.WordCount), http.StatusBadRequest)
	}

	negativePrompt := strings.ReplaceAll(strings.TrimSpace(webCtx.Input("negative_prompt")), "，", ",")
	stylePreset := webCtx.InputWithDefault("style_preset", item.StylePreset)
	if stylePreset == "" {
		stylePreset = "text2img"
	}

	if helper.WordCount(negativePrompt) > 500 {
		return webCtx.JSONError(fmt.Sprintf("排除内容输入字数不能超过 %d", 500), http.StatusBadRequest)
	}

	// 关闭客户端控制宽高的功能，统一由服务端设定，统一计费价格
	width := webCtx.IntInput("width", item.Extension.GetDefaultWidth(512))
	height := webCtx.IntInput("height", item.Extension.GetDefaultHeight(512))

	if width < 1 || height < 1 || width > 2048 || height > 2048 {
		return webCtx.JSONError("invalid width or height", http.StatusBadRequest)
	}

	// AI 自动改写
	aiRewrite := webCtx.InputWithDefault("ai_rewrite", ternary.If(item.Extension.AIRewrite, "true", "false"))

	quota, err := ctl.quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	quotaConsume := int64(coins.GetUnifiedImageGenCoins())

	if evaluate {
		return webCtx.JSON(web.M{"cost": quotaConsume, "enough": quota.Quota >= quota.Used+quotaConsume})
	}

	if quota.Quota < quota.Used+quotaConsume {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	// 内容安全检测
	if checkResp := ctl.securityCheck(webCtx, prompt, user.ID); checkResp != nil {
		return checkResp
	}

	// 加入异步任务队列
	taskID, err := ctl.queue.Enqueue(
		&queue.DeepAICompletionPayload{
			Model:          stylePreset,
			Quota:          quotaConsume,
			UID:            user.ID,
			Prompt:         prompt,
			NegativePrompt: negativePrompt,
			Width:          int64(width),
			Height:         int64(height),
			ImageCount:     1,
			CreatedAt:      time.Now(),
			AIRewrite:      !item.NoPrompt && aiRewrite == "true",
		},
		queue.NewDeepAICompletionTask,
	)
	if err != nil {
		log.Errorf("enqueue task failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}
	log.WithFields(log.Fields{"task_id": taskID}).Debugf("enqueue task success: %s", taskID)

	arguments, _ := json.Marshal(repo.CreativeRecordArguments{
		NegativePrompt: negativePrompt,
		Width:          int64(width),
		Height:         int64(height),
		StylePreset:    stylePreset,
	})

	creativeItem := repo.CreativeItem{
		IslandId:    item.ID,
		IslandType:  repo.IslandTypeImage,
		IslandModel: stylePreset,
		Arguments:   string(arguments),
		Prompt:      prompt,
		TaskId:      taskID,
		Status:      repo.CreativeStatusPending,
	}

	if _, err := ctl.creativeRepo.CreateRecord(ctx, user.ID, &creativeItem); err != nil {
		log.Errorf("create creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"task_id": taskID})
}

// completionsStabilityAI 创作岛项目文本生成 - StabilityAI
func (ctl *CreativeIslandController) completionsStabilityAI(ctx context.Context, webCtx web.Context, item *CreativeIslandItem, user *auth.User, evaluate bool) web.Response {
	prompt := strings.ReplaceAll(strings.TrimSpace(webCtx.Input("prompt")), "，", ",")
	if item.NoPrompt {
		prompt = item.Prompt
	} else {
		if prompt == "" {
			return webCtx.JSONError("prompt is required", http.StatusBadRequest)
		}
	}

	if helper.WordCount(prompt) > item.WordCount {
		return webCtx.JSONError(fmt.Sprintf("创作内容输入字数不能超过 %d", item.WordCount), http.StatusBadRequest)
	}

	negativePrompt := strings.ReplaceAll(strings.TrimSpace(webCtx.Input("negative_prompt")), "，", ",")
	if helper.WordCount(negativePrompt) > 500 {
		return webCtx.JSONError(fmt.Sprintf("排除内容输入字数不能超过 %d", 500), http.StatusBadRequest)
	}

	imageCount := webCtx.Int64Input("image_count", 1)
	if imageCount < 1 || imageCount > 4 {
		return webCtx.JSONError("invalid image count", http.StatusBadRequest)
	}

	var defaultWH int
	if strings.Contains(item.Model, "-1024-") {
		defaultWH = 1024
	} else if strings.Contains(item.Model, "-768-") {
		defaultWH = 768
	} else {
		defaultWH = 512
	}

	width := webCtx.IntInput("width", item.Extension.GetDefaultWidth(defaultWH))
	height := webCtx.IntInput("height", item.Extension.GetDefaultHeight(defaultWH))

	if width < 1 || height < 1 || width > 2048 || height > 2048 {
		return webCtx.JSONError("invalid width or height", http.StatusBadRequest)
	}

	stylePreset := webCtx.InputWithDefault("style_preset", item.StylePreset)

	// 生成步骤数由服务端限制，统一计费价格
	steps := item.Extension.GetDefaultSteps(30)
	// steps := webCtx.IntInput("steps", 50)
	// if !array.In(steps, []int{30, 50, 100, 150}) {
	// 	return webCtx.JSONError("invalid steps", http.StatusBadRequest)
	// }

	image := webCtx.Input("image")
	if item.ModelType == CreativeIslandModelTypeImageToImage {
		if image == "" {
			return webCtx.JSONError("image is required", http.StatusBadRequest)
		}

		if !strings.HasPrefix(image, "http://") && !strings.HasPrefix(image, "https://") {
			return webCtx.JSONError("invalid image", http.StatusBadRequest)
		}
	}

	// AI 自动改写
	aiRewrite := webCtx.InputWithDefault("ai_rewrite", ternary.If(item.Extension.AIRewrite, "true", "false"))

	quota, err := ctl.quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	quotaConsume := int64(coins.GetUnifiedImageGenCoins()) * imageCount
	if evaluate {
		return webCtx.JSON(web.M{"cost": quotaConsume, "enough": quota.Quota >= quota.Used+quotaConsume})
	}

	if quota.Quota < quota.Used+quotaConsume {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	// 内容安全检测
	if checkResp := ctl.securityCheck(webCtx, prompt, user.ID); checkResp != nil {
		return checkResp
	}

	// 加入异步任务队列
	taskID, err := ctl.queue.Enqueue(
		&queue.StabilityAICompletionPayload{
			Model:          item.Model,
			Quota:          quotaConsume,
			UID:            user.ID,
			Prompt:         prompt,
			NegativePrompt: negativePrompt,
			Width:          int64(width),
			Height:         int64(height),
			StylePreset:    stylePreset,
			Steps:          int64(steps),
			Seed:           int64(rand.Intn(100000000)),
			ImageCount:     imageCount,
			Image:          image,
			CreatedAt:      time.Now(),
			AIRewrite:      !item.NoPrompt && aiRewrite == "true",
		},
		queue.NewStabilityAICompletionTask,
	)
	if err != nil {
		log.Errorf("enqueue task failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}
	log.WithFields(log.Fields{"task_id": taskID}).Debugf("enqueue task success: %s", taskID)

	arguments, _ := json.Marshal(repo.CreativeRecordArguments{
		NegativePrompt: negativePrompt,
		Width:          int64(width),
		Height:         int64(height),
		Steps:          int64(steps),
		ImageCount:     imageCount,
		StylePreset:    stylePreset,
		Image:          image,
	})

	creativeItem := repo.CreativeItem{
		IslandId:    item.ID,
		IslandType:  repo.IslandTypeImage,
		IslandModel: item.Model,
		Arguments:   string(arguments),
		Prompt:      prompt,
		TaskId:      taskID,
		Status:      repo.CreativeStatusPending,
	}

	if _, err := ctl.creativeRepo.CreateRecord(ctx, user.ID, &creativeItem); err != nil {
		log.Errorf("create creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"task_id": taskID})
}

// completionsLeapAI 创作岛项目文本生成 - LeapAI
func (ctl *CreativeIslandController) completionsLeapAI(ctx context.Context, webCtx web.Context, item *CreativeIslandItem, user *auth.User, evaluate bool) web.Response {
	prompt := strings.ReplaceAll(strings.TrimSpace(webCtx.Input("prompt")), "，", ",")
	if item.NoPrompt {
		prompt = item.Prompt
	} else {
		if prompt == "" {
			return webCtx.JSONError("prompt is required", http.StatusBadRequest)
		}
	}

	if helper.WordCount(prompt) > item.WordCount {
		return webCtx.JSONError(fmt.Sprintf("创作内容输入字数不能超过 %d", item.WordCount), http.StatusBadRequest)
	}

	negativePrompt := strings.ReplaceAll(strings.TrimSpace(webCtx.Input("negative_prompt")), "，", ",")
	if helper.WordCount(negativePrompt) > 500 {
		return webCtx.JSONError(fmt.Sprintf("排除内容输入字数不能超过 %d", 500), http.StatusBadRequest)
	}

	imageCount := webCtx.Int64Input("image_count", 1)
	if imageCount < 1 || imageCount > 4 {
		return webCtx.JSONError("invalid image count", http.StatusBadRequest)
	}

	// 关闭客户端控制宽高的功能，统一由服务端设定，统一计费价格
	// width, height := 512, 512

	width := webCtx.IntInput("width", item.Extension.GetDefaultWidth(512))
	height := webCtx.IntInput("height", item.Extension.GetDefaultHeight(512))

	if width < 1 || height < 1 || width > 2048 || height > 2048 {
		return webCtx.JSONError("invalid width or height", http.StatusBadRequest)
	}

	// 生成步骤数由服务端限制，统一计费价格
	steps := item.Extension.GetDefaultSteps(30)
	// steps := webCtx.IntInput("steps", 50)
	// if !array.In(steps, []int{30, 50, 100, 150}) {
	// 	return webCtx.JSONError("invalid steps", http.StatusBadRequest)
	// }

	image := webCtx.Input("image")
	if item.ModelType == CreativeIslandModelTypeImageToImage {
		if !evaluate && image == "" {
			return webCtx.JSONError("image is required", http.StatusBadRequest)
		}

		if !evaluate && !strings.HasPrefix(image, "http://") && !strings.HasPrefix(image, "https://") {
			return webCtx.JSONError("invalid image", http.StatusBadRequest)
		}

		// 七牛云图片自动缩放
		image = image + "-thumb"
	}

	// AI 自动改写
	aiRewrite := webCtx.InputWithDefault("ai_rewrite", ternary.If(item.Extension.AIRewrite, "true", "false"))

	quota, err := ctl.quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	quotaConsume := int64(coins.GetUnifiedImageGenCoins()) * imageCount
	if evaluate {
		return webCtx.JSON(web.M{"cost": quotaConsume, "enough": quota.Quota >= quota.Used+quotaConsume})
	}

	if quota.Quota < quota.Used+quotaConsume {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	mode := item.StylePreset
	if mode == "" {
		mode = webCtx.InputWithDefault("style_preset", "canny")
	}

	defaultUpscaleBy := ternary.If(item.Extension.UpscaleBy != "", item.Extension.UpscaleBy, "x1")
	upscaleBy := webCtx.InputWithDefault("upscale_by", defaultUpscaleBy)
	if !array.In(upscaleBy, []string{"x1", "x2", "x4"}) {
		return webCtx.JSONError("invalid upscale_by", http.StatusBadRequest)
	}

	// 内容安全检测
	if checkResp := ctl.securityCheck(webCtx, prompt, user.ID); checkResp != nil {
		return checkResp
	}

	// 加入异步任务队列
	taskID, err := ctl.queue.Enqueue(
		&queue.LeapAICompletionPayload{
			Model:          item.Model,
			Quota:          quotaConsume,
			UID:            user.ID,
			Prompt:         prompt,
			NegativePrompt: negativePrompt,
			Width:          int64(width),
			Height:         int64(height),
			Steps:          int64(steps),
			Seed:           int64(rand.Intn(100000000)),
			ImageCount:     imageCount,
			CreatedAt:      time.Now(),
			Image:          image,
			Mode:           mode,
			AIRewrite:      !item.NoPrompt && aiRewrite == "true",
			UpscaleBy:      upscaleBy,
		},
		queue.NewLeapAICompletionTask,
	)
	if err != nil {
		log.Errorf("enqueue task failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}
	log.WithFields(log.Fields{"task_id": taskID}).Debugf("enqueue task success: %s", taskID)

	arguments, _ := json.Marshal(repo.CreativeRecordArguments{
		NegativePrompt: negativePrompt,
		Width:          int64(width),
		Height:         int64(height),
		Steps:          int64(steps),
		ImageCount:     imageCount,
		StylePreset:    mode,
		Image:          image,
		UpscaleBy:      upscaleBy,
	})

	creativeItem := repo.CreativeItem{
		IslandId:    item.ID,
		IslandType:  repo.IslandTypeImage,
		IslandModel: item.Model,
		Arguments:   string(arguments),
		Prompt:      prompt,
		TaskId:      taskID,
		Status:      repo.CreativeStatusPending,
	}

	if _, err := ctl.creativeRepo.CreateRecord(ctx, user.ID, &creativeItem); err != nil {
		log.Errorf("create creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"task_id": taskID})
}

func (ctl *CreativeIslandController) securityCheck(webCtx web.Context, prompt string, userID int64) web.Response {
	// 内容安全检测
	if checkRes := ctl.securitySrv.PromptDetect(prompt); checkRes != nil {
		if !checkRes.Safe {
			log.WithFields(log.Fields{
				"user_id": userID,
				"details": checkRes,
				"content": prompt,
			}).Warningf("用户 %d 违规，违规内容：%s", userID, checkRes.Reason)
			return webCtx.JSONError("内容违规，已被系统拦截，如有疑问邮件联系：support@aicode.cc", http.StatusNotAcceptable)
		}
	}

	return nil
}

// completionsOpenAI 创作岛项目文本生成 - OpenAI
func (ctl *CreativeIslandController) completionsOpenAI(ctx context.Context, webCtx web.Context, item *CreativeIslandItem, user *auth.User, evaluate bool) web.Response {
	wordCount := webCtx.Int64Input("word_count", 100)
	if wordCount < 1 || wordCount > 4000 {
		return webCtx.JSONError("invalid word count", http.StatusBadRequest)
	}

	prompt := strings.TrimSpace(webCtx.Input("prompt"))
	if item.NoPrompt {
		prompt = ""
	} else {
		if prompt == "" {
			return webCtx.JSONError("prompt is required", http.StatusBadRequest)
		}
	}

	if helper.WordCount(prompt) > item.WordCount {
		return webCtx.JSONError(fmt.Sprintf("创作内容输入字数不能超过 %d", item.WordCount), http.StatusBadRequest)
	}

	var messages []openai.ChatCompletionMessage
	if item.Prompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{Role: "system", Content: fmt.Sprintf("%s。输出内容控制在 %d 个字以内", item.Prompt, wordCount)})
	}
	if prompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{Role: "user", Content: prompt})
	}

	quota, err := ctl.quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, err.Error()), http.StatusInternalServerError)
	}

	// 粗略估算本次请求消耗的 Token 数量，输出内容暂不计费，待实际完成后再计费
	consumeWordCount, _ := openaiHelper.NumTokensFromMessages(messages, item.Model)
	quotaConsumed := coins.GetOpenAITextCoins(item.Model, int64(consumeWordCount))

	if evaluate {
		// 评估时，返回本次请求消耗的 Token 数量，+1 是假定输出内容消耗 1 个智慧果
		return webCtx.JSON(web.M{"cost": quotaConsumed + 1, "enough": quota.Quota >= quota.Used+quotaConsumed+1})
	}

	if quota.Quota < quota.Used+quotaConsumed {
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	// 内容安全检测
	if checkResp := ctl.securityCheck(webCtx, prompt, user.ID); checkResp != nil {
		return checkResp
	}

	// 加入异步任务队列
	taskID, err := ctl.queue.Enqueue(
		&queue.OpenAICompletionPayload{
			Model:     item.Model,
			Quota:     quotaConsumed,
			UID:       user.ID,
			Prompts:   messages,
			WordCount: wordCount,
			CreatedAt: time.Now(),
		},
		queue.NewOpenAICompletionTask,
	)
	if err != nil {
		log.Errorf("enqueue task failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}
	log.WithFields(log.Fields{"task_id": taskID}).Debugf("enqueue task success: %s", taskID)

	arguments, _ := json.Marshal(map[string]any{
		"word_count": wordCount,
	})

	creativeItem := repo.CreativeItem{
		IslandId:    item.ID,
		IslandType:  repo.IslandTypeText,
		IslandModel: item.Model,
		Arguments:   string(arguments),
		Prompt:      prompt,
		TaskId:      taskID,
		Status:      repo.CreativeStatusPending,
	}

	if _, err := ctl.creativeRepo.CreateRecord(ctx, user.ID, &creativeItem); err != nil {
		log.Errorf("create creative item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.trans, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"task_id": taskID})
}

type CreativeIslandCompletionResp struct {
	Type      CreativeIslandCompletionRespType `json:"type"`
	Content   string                           `json:"content"`
	Resources []string                         `json:"resources"`
}

type CreativeIslandCompletionRespType string

const (
	CreativeIslandCompletionRespTypeText         CreativeIslandCompletionRespType = "text"
	CreativeIslandCompletionRespTypeBase64Images CreativeIslandCompletionRespType = "base64-images"
	CreativeIslandCompletionRespTypeUrlImages    CreativeIslandCompletionRespType = "url-images"
)

// CreativeIslandItem 创作岛项目
type CreativeIslandItem struct {
	ID                     string                  `yaml:"id" json:"id"`
	Title                  string                  `yaml:"title" json:"title"`
	TitleColor             string                  `yaml:"title_color,omitempty" json:"title_color,omitempty"`
	Description            string                  `yaml:"description,omitempty" json:"description,omitempty"`
	SupportStream          bool                    `yaml:"support_stream,omitempty" json:"support_stream,omitempty"`
	Vendor                 string                  `yaml:"vendor" json:"vendor"`
	Category               string                  `yaml:"category,omitempty" json:"category,omitempty"`
	ModelType              CreativeIslandModelType `yaml:"model_type" json:"model_type"`
	BgImage                string                  `yaml:"bg_image,omitempty" json:"bg_image,omitempty"`
	BgEmbeddedImage        string                  `yaml:"bg_embedded_image,omitempty" json:"bg_embedded_image,omitempty"`
	Label                  string                  `yaml:"label,omitempty" json:"label,omitempty"`
	LabelColor             string                  `yaml:"label_color,omitempty" json:"label_color,omitempty"`
	WordCount              int64                   `yaml:"word_count,omitempty" json:"word_count,omitempty"`
	Hint                   string                  `yaml:"hint,omitempty" json:"hint,omitempty"`
	StylePreset            string                  `yaml:"style_preset,omitempty" json:"-"`
	SubmitBtnText          string                  `yaml:"submit_btn_text,omitempty" json:"submit_btn_text,omitempty"`
	PromptInputTitle       string                  `yaml:"prompt_input_title,omitempty" json:"prompt_input_title,omitempty"`
	WaitSeconds            int64                   `yaml:"wait_seconds,omitempty" json:"wait_seconds,omitempty"`
	Model                  string                  `yaml:"model" json:"-"`
	Prompt                 string                  `yaml:"prompt,omitempty" json:"-"`
	ShowImageStyleSelector bool                    `yaml:"show_image_style_selector,omitempty" json:"show_image_style_selector,omitempty"`
	NoPrompt               bool                    `yaml:"no_prompt,omitempty" json:"no_prompt,omitempty"`

	Extension repo.CreativeIslandExt `yaml:"extension" json:"extension"`
}

func CreativeIslandItemFromModel(item repo.CreativeIsland) CreativeIslandItem {
	wordCount := item.WordCount
	if wordCount <= 0 {
		wordCount = 1000
	}

	waitSeconds := item.WaitSeconds
	if waitSeconds <= 0 {
		waitSeconds = 30
	}

	return CreativeIslandItem{
		ID:                     item.IslandId,
		Title:                  item.Title,
		TitleColor:             item.TitleColor,
		Description:            item.Description,
		SupportStream:          false,
		Vendor:                 item.Vendor,
		Model:                  item.Model,
		StylePreset:            item.StylePreset,
		Category:               item.Category,
		BgImage:                item.BgImage,
		BgEmbeddedImage:        item.BgEmbeddedImage,
		ModelType:              CreativeIslandModelType(item.ModelType),
		Label:                  item.Label,
		LabelColor:             item.LabelColor,
		SubmitBtnText:          item.SubmitBtnText,
		PromptInputTitle:       item.PromptInputTitle,
		WaitSeconds:            waitSeconds,
		WordCount:              wordCount,
		Hint:                   item.Hint,
		Prompt:                 item.Prompt,
		ShowImageStyleSelector: item.ShowImageStyleSelector == 1,
		NoPrompt:               item.NoPrompt == 1,
		Extension:              item.Extension.Init(),
	}
}

type CreativeIslandModelType string

const (
	CreativeIslandModelTypeText         CreativeIslandModelType = "text-generation"
	CreativeIslandModelTypeImage        CreativeIslandModelType = "image-generation"
	CreativeIslandModelTypeImageToImage CreativeIslandModelType = "image-to-image"
)
