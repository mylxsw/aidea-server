package baidu

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) *BaiduAI {
		return NewBaiduAI(conf.BaiduWXKey, conf.BaiduWXSecret)
	})
}
