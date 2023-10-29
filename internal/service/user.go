package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/rate"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/repo/model"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/must"
	"github.com/redis/go-redis/v9"
)

type UserService struct {
	userRepo  *repo.UserRepo
	quotaRepo *repo.QuotaRepo
	rds       *redis.Client
	limiter   *rate.RateLimiter
	conf      *config.Config
}

func NewUserService(conf *config.Config, userRepo *repo.UserRepo, quotaRepo *repo.QuotaRepo, rds *redis.Client, limiter *rate.RateLimiter) *UserService {
	return &UserService{conf: conf, userRepo: userRepo, quotaRepo: quotaRepo, rds: rds, limiter: limiter}
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

	secondsRemain := helper.TodayRemainTimeSeconds()
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

// CustomConfig 获取用户自定义配置
func (srv *UserService) CustomConfig(ctx context.Context, userID int64) (*repo.UserCustomConfig, error) {
	return srv.userRepo.CustomConfig(ctx, userID)
}

// UpdateCustomConfig 更新用户自定义配置
func (srv *UserService) UpdateCustomConfig(ctx context.Context, userID int64, config repo.UserCustomConfig) error {
	return srv.userRepo.UpdateCustomConfig(ctx, userID, config)
}
