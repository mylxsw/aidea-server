package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/config"
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
	Provider360        = "360智脑"
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
	Dynamic bool   `json:"dynamic"`
}

// ChannelTypes 支持的渠道类型列表
func (svc *ChatService) ChannelTypes() []ChannelType {
	return []ChannelType{
		{Name: ProviderOpenAI, Dynamic: true},
		{Name: ProviderOneAPI, Dynamic: true},
		{Name: ProviderOpenRouter, Dynamic: true},

		{Name: ProviderXunFei, Dynamic: false},
		{Name: ProviderWenXin, Dynamic: false},
		{Name: ProviderDashscope, Dynamic: false},
		{Name: ProviderSenseNova, Dynamic: false},
		{Name: ProviderTencent, Dynamic: false},
		{Name: ProviderBaiChuan, Dynamic: false},
		{Name: Provider360, Dynamic: false},
		{Name: ProviderSky, Dynamic: false},
		{Name: ProviderZhipu, Dynamic: false},
		{Name: ProviderMoonshot, Dynamic: false},
		{Name: ProviderGoogle, Dynamic: false},
		{Name: ProviderAnthropic, Dynamic: false},
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

	return false
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
