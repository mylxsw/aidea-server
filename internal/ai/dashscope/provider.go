package dashscope

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/array"
	"strings"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) *DashScope {
		keys := append(conf.DashScopeKeys, conf.DashScopeKey)
		keys = array.Filter(keys, func(key string, _ int) bool {
			return strings.TrimSpace(key) != ""
		})

		return New(array.Distinct(keys)...)
	})
}
