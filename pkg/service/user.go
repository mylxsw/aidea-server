package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/rate"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"strconv"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/must"
	"github.com/redis/go-redis/v9"
)

type UserService struct {
	userRepo  *repo.UserRepo
	roomRepo  *repo.RoomRepo
	quotaRepo *repo.QuotaRepo
	rds       *redis.Client
	limiter   *rate.RateLimiter
	conf      *config.Config
}

func NewUserService(conf *config.Config, userRepo *repo.UserRepo, roomRepo *repo.RoomRepo, quotaRepo *repo.QuotaRepo, rds *redis.Client, limiter *rate.RateLimiter) *UserService {
	return &UserService{conf: conf, userRepo: userRepo, roomRepo: roomRepo, quotaRepo: quotaRepo, rds: rds, limiter: limiter}
}

type FreeChatState struct {
	coins.ModelWithName
	LeftCount int `json:"left_count"`
	MaxCount  int `json:"max_count"`
}

// FreeChatStatistics 用户免费聊天次数统计
func (srv *UserService) FreeChatStatistics(ctx context.Context, userID int64) []FreeChatState {
	return array.Map(coins.FreeModels(), func(item coins.ModelWithName, _ int) FreeChatState {
		leftCount, maxCount := srv.FreeChatRequestCounts(ctx, userID, item.Model)
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
func (srv *UserService) FreeChatStatisticsForModel(ctx context.Context, userID int64, model string) (*FreeChatState, error) {
	realModel := model
	if srv.conf.VirtualModel.NanxianRel != "" && realModel == chat.ModelNanXian {
		realModel = srv.conf.VirtualModel.NanxianRel
	}

	if srv.conf.VirtualModel.BeichouRel != "" && realModel == chat.ModelBeiChou {
		realModel = srv.conf.VirtualModel.BeichouRel
	}

	freeModel := coins.GetFreeModel(realModel)
	if freeModel == nil || freeModel.FreeCount <= 0 {
		return nil, ErrorModelNotFree
	}

	// 填充免费模型名称
	freeModel.Model = model

	leftCount, maxCount := srv.FreeChatRequestCounts(ctx, userID, realModel)
	return &FreeChatState{
		ModelWithName: *freeModel,
		LeftCount:     leftCount,
		MaxCount:      maxCount,
	}, nil
}

func (srv *UserService) freeChatCacheKey(userID int64, model string) string {
	return fmt.Sprintf("free-chat:uid:%d:model:%s", userID, model)
}

// FreeChatRequestCounts 免费模型使用次数：每天免费 n 次
func (srv *UserService) FreeChatRequestCounts(ctx context.Context, userID int64, model string) (leftCount int, maxCount int) {
	if srv.conf.VirtualModel.NanxianRel != "" && model == chat.ModelNanXian {
		model = srv.conf.VirtualModel.NanxianRel
	}

	if srv.conf.VirtualModel.BeichouRel != "" && model == chat.ModelBeiChou {
		model = srv.conf.VirtualModel.BeichouRel
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	freeModel := coins.GetFreeModel(model)
	if freeModel != nil && freeModel.FreeCount > 0 {
		leftCount, maxCount = freeModel.FreeCount, freeModel.FreeCount

		optCount, err := srv.limiter.OperationCount(ctx, srv.freeChatCacheKey(userID, model))
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
func (srv *UserService) UpdateFreeChatCount(ctx context.Context, userID int64, model string) error {
	if srv.conf.VirtualModel.NanxianRel != "" && model == chat.ModelNanXian {
		model = srv.conf.VirtualModel.NanxianRel
	}

	if srv.conf.VirtualModel.BeichouRel != "" && model == chat.ModelBeiChou {
		model = srv.conf.VirtualModel.BeichouRel
	}

	if !coins.IsFreeModel(model) {
		return nil
	}

	secondsRemain := misc.TodayRemainTimeSeconds()
	if err := srv.limiter.OperationIncr(
		ctx,
		srv.freeChatCacheKey(userID, model),
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

// GetUserByID 根据用户ID获取用户信息，带缓存（10分钟）
func (srv *UserService) GetUserByID(ctx context.Context, id int64, forceUpdate bool) (*model.Users, error) {
	// 注意：在用户绑定手机号的时候会自动清空当前缓存
	userKey := fmt.Sprintf("user:%d:info", id)

	if !forceUpdate {
		if user, err := srv.rds.Get(ctx, userKey).Result(); err == nil {
			var u model.Users
			if err := json.Unmarshal([]byte(user), &u); err == nil {
				return &u, nil
			}
		}
	}

	user, err := srv.userRepo.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := srv.rds.SetNX(ctx, userKey, string(must.Must(json.Marshal(user))), 10*time.Minute).Err(); err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByAPIKey 根据用户 API Key 获取用户信息，带缓存（10分钟）
func (srv *UserService) GetUserByAPIKey(ctx context.Context, key string) (*model.Users, error) {
	userKey := fmt.Sprintf("user-apikey:%s:info", key)
	user, err := srv.userRepo.GetUserByAPIKey(ctx, key)
	if err != nil {
		return nil, err
	}

	if err := srv.rds.SetNX(ctx, userKey, string(must.Must(json.Marshal(user))), 10*time.Minute).Err(); err != nil {
		return nil, err
	}

	return user, nil
}

// CustomConfig 获取用户自定义配置
func (srv *UserService) CustomConfig(ctx context.Context, userID int64) (*repo.UserCustomConfig, error) {
	return srv.userRepo.CustomConfig(ctx, userID)
}

// UpdateCustomConfig 更新用户自定义配置
func (srv *UserService) UpdateCustomConfig(ctx context.Context, userID int64, config repo.UserCustomConfig) error {
	return srv.userRepo.UpdateCustomConfig(ctx, userID, config)
}

// UserQuota 用户配额
type UserQuota struct {
	Quota   int64 `json:"quota"`
	Used    int64 `json:"used"`
	Rest    int64 `json:"rest"`
	Freezed int64 `json:"freezed"`
}

// UserQuota 获取用户配额
func (srv *UserService) UserQuota(ctx context.Context, userID int64) (*UserQuota, error) {
	quota, err := srv.quotaRepo.GetUserQuota(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user quota failed: %w", err)
	}

	freezed, err := srv.rds.Get(ctx, srv.userQuotaFreezedCacheKey(userID)).Int()
	if err != nil && err != redis.Nil {
		log.F(log.M{"user_id": userID, "quota": quota}).Errorf("查询用户冻结的配额失败: %s", err)

		return &UserQuota{Rest: quota.Rest, Quota: quota.Quota, Used: quota.Used}, nil
	}

	return &UserQuota{
		Rest:    quota.Rest,
		Quota:   quota.Quota,
		Used:    quota.Used,
		Freezed: int64(freezed),
	}, nil
}

// FreezeUserQuota 冻结用户配额
func (srv *UserService) FreezeUserQuota(ctx context.Context, userID int64, quota int64) error {
	if quota <= 0 {
		return nil
	}

	key := srv.userQuotaFreezedCacheKey(userID)
	_, err := srv.rds.IncrBy(ctx, key, quota).Result()
	if err != nil {
		return fmt.Errorf("freeze user quota failed: %w", err)
	}

	if err := srv.rds.Expire(ctx, key, 5*time.Minute).Err(); err != nil {
		log.F(log.M{"user_id": userID, "quota": quota}).Errorf("设置用户冻结配额过期时间失败: %s", err)
	}

	return nil
}

// UnfreezeUserQuota 解冻用户配额
func (srv *UserService) UnfreezeUserQuota(ctx context.Context, userID int64, quota int64) error {
	if quota <= 0 {
		return nil
	}

	key := srv.userQuotaFreezedCacheKey(userID)
	newVal, err := srv.rds.DecrBy(ctx, key, quota).Result()
	if err != nil {
		return fmt.Errorf("解冻用户配额失败: %w", err)
	}

	if newVal <= 0 {
		if err := srv.rds.Del(ctx, key).Err(); err != nil {
			log.F(log.M{"user_id": userID, "quota": quota}).Errorf("清空用户冻结配额失败: %s", err)
		}
	}

	return nil
}

func (srv *UserService) userQuotaFreezedCacheKey(userID int64) string {
	return fmt.Sprintf("user:%d:quota:freezed", userID)
}

type HomeModel struct {
	// Type 模型类型：支持 model/room_gallery/rooms/room_enterprise
	Type          string `json:"type"`
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	AvatarURL     string `json:"avatar_url,omitempty"`
	ModelID       string `json:"model_id,omitempty"`
	ModelName     string `json:"model_name,omitempty"`
	SupportVision bool   `json:"support_vision,omitempty"`
	Prompt        string `json:"-"`
}

const (
	HomeModelTypeModel          = "model"
	HomeModelTypeRoomGallery    = "room_gallery"
	HomeModelTypeRooms          = "rooms"
	HomeModelTypeRoomEnterprise = "room_enterprise"
)

func (srv *UserService) QueryHomeModel(ctx context.Context, models map[string]chat.Model, userID int64, homeModelUniqueKey string) (*HomeModel, error) {
	segs := strings.SplitN(homeModelUniqueKey, "|", 2)
	if len(segs) != 2 {
		return nil, fmt.Errorf("invalid home model format")
	}

	res := HomeModel{}
	res.Type, res.ID = segs[0], segs[1]

	switch res.Type {
	case HomeModelTypeRoomGallery:
		room, err := srv.roomRepo.GalleryItem(ctx, int64(must.Must(strconv.Atoi(res.ID))))
		if err != nil {
			return nil, fmt.Errorf("get room gallery item failed: %v", err)
		}

		res.Name = room.Name
		res.ModelID = room.Model
		res.Prompt = room.Prompt
		res.AvatarURL = room.AvatarUrl
		mod, ok := models[room.Vendor+":"+room.Model]
		if ok {
			res.SupportVision = mod.SupportVision
			res.ModelName = mod.Name
		}
	case HomeModelTypeRooms:
		room, err := srv.roomRepo.Room(ctx, userID, int64(must.Must(strconv.Atoi(res.ID))))
		if err != nil {
			return nil, fmt.Errorf("get room item failed: %v", err)
		}

		res.Name = room.Name
		res.ModelID = room.Model
		res.Prompt = room.SystemPrompt
		res.AvatarURL = room.AvatarUrl
		mod, ok := models[room.Vendor+":"+room.Model]
		if ok {
			res.SupportVision = mod.SupportVision
			res.ModelID = room.Model
		}
	case HomeModelTypeModel:
		mod, ok := models[res.ID]
		if !ok {
			segs := strings.Split(res.ID, ":")
			mod, ok = models[segs[len(segs)-1]]
			if !ok {
				return nil, fmt.Errorf("model not found: %s", res.ID)
			}
		}

		res.Name = mod.ShortName
		res.ModelID = mod.ID
		res.SupportVision = mod.SupportVision
		res.AvatarURL = mod.AvatarURL
	}

	return &res, nil
}
