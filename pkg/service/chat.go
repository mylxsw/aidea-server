package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/rate"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/must"
	"github.com/mylxsw/go-utils/ternary"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

type ChatService struct {
	conf    *config.Config    `autowire:"@"`
	rds     *redis.Client     `autowire:"@"`
	limiter *rate.RateLimiter `autowire:"@"`
	rep     *repo.Repository  `autowire:"@"`
}

func NewChatService(resolver infra.Resolver) *ChatService {
	svc := &ChatService{}
	resolver.MustAutoWire(svc)
	return svc
}

func (svc *ChatService) Room(ctx context.Context, userID int64, roomID int64) (*model.Rooms, error) {
	roomKey := fmt.Sprintf("chat-room:%d:%d:info", userID, roomID)
	if roomStr, err := svc.rds.Get(ctx, roomKey).Result(); err == nil {
		var room model.Rooms
		if err := json.Unmarshal([]byte(roomStr), &room); err == nil {
			return &room, nil
		}
	}

	room, err := svc.rep.Room.Room(ctx, userID, roomID)
	if err != nil {
		return nil, err
	}

	if err := svc.rds.SetNX(ctx, roomKey, string(must.Must(json.Marshal(room))), 60*time.Minute).Err(); err != nil {
		return nil, err
	}

	return room, nil
}

const (
	ProviderOpenAI     = "openai"
	ProviderXunFei     = "讯飞星火"
	ProviderWenXin     = "文心千帆"
	ProviderDashscope  = "灵积"
	ProviderSenseNova  = "商汤日日新"
	ProviderTencent    = "腾讯"
	ProviderBaiChuan   = "百川"
	Provider360        = "360"
	ProviderOneAPI     = "oneapi"
	ProviderOpenRouter = "openrouter"
	ProviderSky        = "sky"
	ProviderZhipu      = "zhipu"
	ProviderMoonshot   = "moonshot"
	ProviderGoogle     = "google"
	ProviderAnthropic  = "Anthropic"
)

type ChannelType struct {
	Name    string `json:"name"`
	Display string `json:"display,omitempty"`
	Dynamic bool   `json:"dynamic"`
}

// ChannelTypes 支持的渠道类型列表
func (svc *ChatService) ChannelTypes() []ChannelType {
	return []ChannelType{
		{Name: ProviderOpenAI, Dynamic: true, Display: "OpenAI"},
		{Name: ProviderOneAPI, Dynamic: true, Display: "OneAPI"},
		{Name: ProviderOpenRouter, Dynamic: true, Display: "OpenRouter"},

		{Name: ProviderXunFei, Dynamic: false, Display: "讯飞星火"},
		{Name: ProviderWenXin, Dynamic: false, Display: "文心千帆"},
		{Name: ProviderDashscope, Dynamic: false, Display: "阿里灵积"},
		{Name: ProviderSenseNova, Dynamic: false, Display: "商汤"},
		{Name: ProviderTencent, Dynamic: false, Display: "腾讯"},
		{Name: ProviderBaiChuan, Dynamic: false, Display: "百川"},
		{Name: Provider360, Dynamic: false, Display: "360"},
		{Name: ProviderSky, Dynamic: false, Display: "昆仑万维"},
		{Name: ProviderZhipu, Dynamic: false, Display: "智谱"},
		{Name: ProviderMoonshot, Dynamic: false, Display: "月之暗面"},
		{Name: ProviderGoogle, Dynamic: false, Display: "Google"},
		{Name: ProviderAnthropic, Dynamic: false, Display: "Anthropic"},
	}
}

// TODO 缓存
func (svc *ChatService) Models(ctx context.Context, returnAll bool) []repo.Model {
	models, err := svc.rep.Model.GetModels(ctx)
	if err != nil {
		log.Errorf("get models failed: %v", err)
		return nil
	}

	models = array.Map(models, func(m repo.Model, _ int) repo.Model {
		m.Status = ternary.If(svc.isModelEnabled(m), repo.ModelStatusEnabled, repo.ModelStatusDisabled)
		return m
	})

	return array.Filter(models, func(item repo.Model, _ int) bool {
		if returnAll {
			return true
		}

		return item.Status == repo.ModelStatusEnabled
	})
}

func PureModelID(modelID string) string {
	segs := strings.SplitN(modelID, ":", 2)
	if len(segs) > 1 {
		return segs[1]
	}

	return modelID
}

// TODO 缓存
func (svc *ChatService) Model(ctx context.Context, modelID string) *repo.Model {
	modelID = PureModelID(modelID)

	ret, err := svc.rep.Model.GetModel(ctx, modelID)
	if err != nil {
		log.Errorf("get model %s failed: %v", modelID, err)
		return nil
	}

	ret.Status = ternary.If(svc.isModelEnabled(*ret), repo.ModelStatusEnabled, repo.ModelStatusDisabled)
	return ret
}

// isModelEnabled 判断模型是否启用
func (svc *ChatService) isModelEnabled(item repo.Model) bool {
	if item.Status == repo.ModelStatusDisabled {
		return false
	}

	if svc.conf.EnableOpenAI && item.SupportProvider(ProviderOpenAI) != nil {
		return true
	}

	if svc.conf.EnableXFYunAI && item.SupportProvider(ProviderXunFei) != nil {
		return true
	}

	if svc.conf.EnableBaiduWXAI && item.SupportProvider(ProviderWenXin) != nil {
		return true
	}

	if svc.conf.EnableDashScopeAI && item.SupportProvider(ProviderDashscope) != nil {
		return true
	}

	if svc.conf.EnableSenseNovaAI && item.SupportProvider(ProviderSenseNova) != nil {
		return true
	}

	if svc.conf.EnableTencentAI && item.SupportProvider(ProviderTencent) != nil {
		return true
	}

	if svc.conf.EnableBaichuan && item.SupportProvider(ProviderBaiChuan) != nil {
		return true

	}

	if svc.conf.EnableGPT360 && item.SupportProvider(Provider360) != nil {
		return true
	}

	if svc.conf.EnableOneAPI && item.SupportProvider(ProviderOneAPI) != nil {
		return true
	}

	if svc.conf.EnableOpenRouter && item.SupportProvider(ProviderOpenRouter) != nil {
		return true
	}

	if svc.conf.EnableSky && item.SupportProvider(ProviderSky) != nil {
		return true
	}

	if svc.conf.EnableZhipuAI && item.SupportProvider(ProviderZhipu) != nil {
		return true
	}

	if svc.conf.EnableMoonshot && item.SupportProvider(ProviderMoonshot) != nil {
		return true
	}

	if svc.conf.EnableGoogleAI && item.SupportProvider(ProviderGoogle) != nil {
		return true
	}

	if svc.conf.EnableAnthropic && item.SupportProvider(ProviderAnthropic) != nil {
		return true
	}

	return item.SupportDynamicProvider()
}

// Channels 返回所有支持的渠道
// TODO 缓存
func (svc *ChatService) Channels(ctx context.Context) ([]repo.Channel, error) {
	return svc.rep.Model.GetChannels(ctx)
}

// Channel 返回制定的渠道信息
// TODO 缓存
func (svc *ChatService) Channel(ctx context.Context, id int64) (*repo.Channel, error) {
	return svc.rep.Model.GetChannel(ctx, id)
}

// DailyFreeModels 返回每日免费模型列表
func (svc *ChatService) DailyFreeModels(ctx context.Context) ([]coins.ModelWithName, error) {
	models, err := svc.rep.Model.DailyFreeModels(ctx)
	if err != nil {
		return nil, err
	}

	return array.UniqBy(append(coins.FreeModels(), array.Map(models, func(item model.ModelsDailyFree, _ int) coins.ModelWithName {
		return coins.ModelWithName{
			ID:        item.Id,
			Model:     item.ModelId,
			Name:      item.Name,
			Info:      item.Info,
			FreeCount: int(item.FreeCount),
			EndAt:     item.EndAt,
		}
	})...), func(item coins.ModelWithName) string {
		return item.Model
	}), nil
}

// GetDailyFreeModel 获取每日免费模型信息
func (svc *ChatService) GetDailyFreeModel(ctx context.Context, modelId string) (*coins.ModelWithName, error) {
	res := coins.GetFreeModel(modelId)
	if res != nil {
		return res, nil
	}

	item, err := svc.rep.Model.GetDailyFreeModel(ctx, modelId)
	if err != nil {
		return nil, err
	}

	return &coins.ModelWithName{
		ID:        item.Id,
		Model:     item.ModelId,
		Name:      item.Name,
		Info:      item.Info,
		FreeCount: int(item.FreeCount),
		EndAt:     item.EndAt,
	}, nil
}

type FreeChatState struct {
	coins.ModelWithName
	LeftCount int `json:"left_count"`
	MaxCount  int `json:"max_count"`
}

// FreeChatStatistics 用户免费聊天次数统计
func (svc *ChatService) FreeChatStatistics(ctx context.Context, userID int64) []FreeChatState {
	freeModels, err := svc.DailyFreeModels(ctx)
	if err != nil {
		log.Errorf("get daily free models failed: %v", err)
		return nil
	}

	return array.Map(freeModels, func(item coins.ModelWithName, _ int) FreeChatState {
		leftCount, maxCount := svc.freeChatRequestCounts(ctx, userID, &item)
		return FreeChatState{
			ModelWithName: item,
			LeftCount:     leftCount,
			MaxCount:      maxCount,
		}
	})
}

var (
	ErrorModelNotFree = fmt.Errorf("model is not free")
)

// FreeChatStatisticsForModel 用户免费聊天次数统计
func (svc *ChatService) FreeChatStatisticsForModel(ctx context.Context, userID int64, model string) (*FreeChatState, error) {
	freeModel, err := svc.GetDailyFreeModel(ctx, model)
	if err != nil || freeModel.FreeCount <= 0 {
		return nil, ErrorModelNotFree
	}

	leftCount, maxCount := svc.freeChatRequestCounts(ctx, userID, freeModel)
	return &FreeChatState{
		ModelWithName: *freeModel,
		LeftCount:     leftCount,
		MaxCount:      maxCount,
	}, nil
}

func (svc *ChatService) freeChatCacheKey(userID int64, model string) string {
	return fmt.Sprintf("free-chat:uid:%d:model:%s", userID, model)
}

func (svc *ChatService) FreeChatRequestCounts(ctx context.Context, userID int64, modelId string) (leftCount int, maxCount int) {
	mod, _ := svc.GetDailyFreeModel(ctx, modelId)
	return svc.freeChatRequestCounts(ctx, userID, mod)
}

// freeChatRequestCounts 免费模型使用次数：每天免费 n 次
func (svc *ChatService) freeChatRequestCounts(ctx context.Context, userID int64, model *coins.ModelWithName) (leftCount int, maxCount int) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if model != nil && model.FreeCount > 0 {
		leftCount, maxCount = model.FreeCount, model.FreeCount

		optCount, err := svc.limiter.OperationCount(ctx, svc.freeChatCacheKey(userID, model.Model))
		if err != nil {
			log.WithFields(log.Fields{
				"user_id": userID,
				"model":   model,
			}).Errorf("get chat operation count failed: %s", err)
		}

		leftCount = maxCount - int(optCount)
		if leftCount < 0 {
			leftCount = 0
		}

		return leftCount, maxCount
	} else {
		leftCount, maxCount = 0, 0
	}

	return leftCount, maxCount
}

// UpdateFreeChatCount 更新免费聊天次数使用情况
func (svc *ChatService) UpdateFreeChatCount(ctx context.Context, userID int64, model string) error {
	_, err := svc.GetDailyFreeModel(ctx, model)
	if err != nil {
		return err
	}

	secondsRemain := misc.TodayRemainTimeSeconds()
	if err := svc.limiter.OperationIncr(
		ctx,
		svc.freeChatCacheKey(userID, model),
		time.Duration(secondsRemain)*time.Second,
	); err != nil {
		log.WithFields(log.Fields{
			"user_id": userID,
			"model":   model,
		}).Errorf("incr chat operation count failed: %s", err)

		return err
	}

	return nil
}
