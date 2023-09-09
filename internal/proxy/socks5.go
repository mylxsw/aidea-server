package proxy

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
	"golang.org/x/net/proxy"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) (proxy.Dialer, error) {
		return proxy.SOCKS5("tcp", conf.Socks5Proxy, nil, proxy.Direct)
	})
}

func (Provider) ShouldLoad(conf *config.Config) bool {
	return conf.Socks5Proxy != ""
}
