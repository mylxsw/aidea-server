package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/repo/model"
	"github.com/mylxsw/go-utils/must"
	"github.com/redis/go-redis/v9"
)

type UserService struct {
	userRepo  *repo.UserRepo
	quotaRepo *repo.QuotaRepo
	rds       *redis.Client
}

func NewUserService(userRepo *repo.UserRepo, quotaRepo *repo.QuotaRepo, rds *redis.Client) *UserService {
	return &UserService{userRepo: userRepo, quotaRepo: quotaRepo, rds: rds}
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
