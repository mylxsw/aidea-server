package rate

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

var ErrRateLimitExceeded = errors.New("请求频率过高，请稍后再试")

func NewLimiter(rdb *redis.Client) *redis_rate.Limiter {
	return redis_rate.NewLimiter(rdb)
}

func MaxRequestsInPeriod(count int, period time.Duration) redis_rate.Limit {
	return redis_rate.Limit{Rate: count, Burst: count, Period: period}
}

type RateLimiter struct {
	limiter *redis_rate.Limiter
	rds     *redis.Client
}

func New(rds *redis.Client, limiter *redis_rate.Limiter) *RateLimiter {
	return &RateLimiter{limiter: limiter, rds: rds}
}

// Allow 检查是否允许访问
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit redis_rate.Limit) error {
	res, err := rl.limiter.Allow(ctx, key, limit)
	if err != nil {
		return err
	}

	if res.Remaining <= 0 {
		return ErrRateLimitExceeded
	}

	return nil
}

// OperationCount 获取操作次数
func (rl *RateLimiter) OperationCount(ctx context.Context, key string) (int64, error) {
	res, err := rl.rds.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}

		return 0, err
	}

	if res == "" {
		return 0, nil
	}

	return strconv.ParseInt(res, 10, 64)
}

// OperationIncr 操作次数增加
func (rl *RateLimiter) OperationIncr(ctx context.Context, key string, ttl time.Duration) error {
	_, err := rl.rds.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	_, err = rl.rds.Expire(ctx, key, ttl).Result()
	return err
}
