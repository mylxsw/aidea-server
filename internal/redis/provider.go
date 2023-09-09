package redis

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
	"github.com/redis/go-redis/v9"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) *redis.Client {
		return redis.NewClient(&redis.Options{
			Addr:     conf.RedisAddr(),
			Password: conf.RedisPassword,
		})
	})
}
