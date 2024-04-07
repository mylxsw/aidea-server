package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/redis/go-redis/v9"
	"time"
)

type SettingService struct {
	conf *config.Config   `autowire:"@"`
	repo *repo.Repository `autowire:"@"`
	rds  *redis.Client    `autowire:"@"`
}

func NewSettingService(resolver infra.Resolver) *SettingService {
	svc := &SettingService{}
	resolver.MustAutoWire(svc)
	return svc
}

// Get the specified configuration item.
func (svc *SettingService) Get(context context.Context, key string) (string, error) {
	settingKey := "dynamic-setting:" + key
	if value, err := svc.rds.Get(context, settingKey).Result(); err == nil {
		return value, nil
	}

	value, err := svc.repo.Setting.Get(context, key)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return "", err
	}

	svc.rds.Set(context, settingKey, value, 24*time.Hour)
	return value, nil
}

// ReloadKey Reload the specified configuration item.
func (svc *SettingService) ReloadKey(context context.Context, key string) error {
	settingKey := "dynamic-setting:" + key
	value, err := svc.repo.Setting.Get(context, key)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return err
	}

	svc.rds.Set(context, settingKey, value, 24*time.Hour)
	return nil
}

func (svc *SettingService) clearDynamicSettings(ctx context.Context) error {
	var cursor uint64
	prefix := "dynamic-setting:"

	for {
		var keys []string
		var err error

		keys, cursor, err = svc.rds.Scan(ctx, cursor, prefix+"*", 10).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if _, err = svc.rds.Del(ctx, keys...).Result(); err != nil {
				return err
			}
		}

		if cursor == 0 {
			break
		}
	}

	return nil
}

// ReloadAll Reload all configuration items.
func (svc *SettingService) ReloadAll(context context.Context) error {
	// Clear cache for all configuration items
	if err := svc.clearDynamicSettings(context); err != nil {
		return err
	}

	// Reload all configuration items
	settings, err := svc.repo.Setting.All(context)
	if err != nil {
		return err
	}

	for _, s := range settings {
		svc.rds.Set(context, "dynamic-setting:"+s.Key, s.Value, 24*time.Hour)
	}

	return nil
}

// Avatars Get the list of avatars.
func (svc *SettingService) Avatars(ctx context.Context) []string {
	data, err := svc.Get(ctx, "avatars")
	if err != nil {
		log.F(log.M{"key": "avatars"}).Errorf("get avatars setting failed: %s", err)
		return []string{}
	}

	var avatars []string
	if err := json.Unmarshal([]byte(data), &avatars); err != nil {
		log.F(log.M{"data": data}).Errorf("parse avatars setting failed: %s", err)
		return avatars
	}

	return avatars
}
