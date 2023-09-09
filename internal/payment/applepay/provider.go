package applepay

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) ApplePay {
		if !conf.EnableAlipay {
			return &ApplePayFake{}
		}

		return NewApplePay()
	})
}
