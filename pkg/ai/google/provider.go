package google

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(resolver infra.Resolver, conf *config.Config) *GoogleAI {
		return newGoogleAI(resolver, conf)
	})
}
