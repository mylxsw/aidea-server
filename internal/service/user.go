package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
}

func NewUserService(userRepo *repo.UserRepo, quotaRepo *repo.QuotaRepo, rds *redis.Client, limiter *rate.RateLimiter) *UserService {
	return &UserService{userRepo: userRepo, quotaRepo: quotaRepo, rds: rds, limiter: limiter}
}

const maxFreeCount = 15

type FreeChatState struct {
	coins.ModelWithName
	LeftCount int `json:"left_count"`
	MaxCount  int `json:"max_count"`
}

// FreeChatStatistics 用户免费聊天次数统计
func (src *UserService) FreeChatStatistics(ctx context.Context, userID int64) []FreeChatState {
	return array.Map(coins.FreeModels(), func(item coins.ModelWithName, _ int) FreeChatState {
		leftCount, maxCount := src.FreeChatRequestCounts(ctx, userID, item.Model)
		return FreeChatState{
			ModelWithName: item,
			LeftCount:     leftCount,
			MaxCount:      maxCount,
		}
	})
}

// FreeChatStatisticsForModel 用户免费聊天次数统计
func (src *UserService) FreeChatStatisticsForModel(ctx context.Context, userID int64, model string) (*FreeChatState, error) {
	freeModel := coins.GetFreeModel(model)
	if freeModel == nil {
		return nil, fmt.Errorf("model %s is not free", model)
	}

	leftCount, maxCount := src.FreeChatRequestCounts(ctx, userID, model)
	return &FreeChatState{
		ModelWithName: *freeModel,
		LeftCount:     leftCount,
		MaxCount:      maxCount,
	}, nil
}

func (src *UserService) freeChatCacheKey(userID int64, model string) string {
	return fmt.Sprintf("free-chat:uid:%d:model:%s", userID, model)
}

// FreeChatRequestCounts 免费模型使用次数：每天免费 15 次
func (src *UserService) FreeChatRequestCounts(ctx context.Context, userID int64, model string) (leftCount int, maxCount int) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if coins.IsFreeModel(model) {
		leftCount, maxCount = maxFreeCount, maxFreeCount

		optCount, err := src.limiter.OperationCount(ctx, src.freeChatCacheKey(userID, model))
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

	return leftCount, maxFreeCount
}

// UpdateFreeChatCount 更新免费聊天次数使用情况
func (src *UserService) UpdateFreeChatCount(ctx context.Context, userID int64, model string) error {
	if !coins.IsFreeModel(model) {
		return nil
	}

	secondsRemain := helper.TodayRemainTimeSeconds()
	if err := src.limiter.OperationIncr(
		ctx,
		src.freeChatCacheKey(userID, model),
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
