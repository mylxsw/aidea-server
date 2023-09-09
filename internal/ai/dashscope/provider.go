package dashscope

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) *DashScope {
		return New(conf.DashScopeKey)
	})
}
