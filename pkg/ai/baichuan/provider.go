package baichuan

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) *BaichuanAI {
		return NewBaichuanAI(conf.BaichuanAPIKey, conf.BaichuanSecret)
	})
}
