package youdao

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, resolver infra.Resolver) *Client {
		return NewClient(conf.TranslateServer, conf.TranslateAPPID, conf.TranslateAPPKey)
	})

	binder.MustSingleton(func(conf *config.Config, cacheRepo *repo.CacheRepo, client *Client) Translater {
		if conf.EnableTranslate {
			return NewTranslater(cacheRepo, client)
		}

		return &FakeTranslater{}
	})
}
