package proxy

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/ternary"
	"golang.org/x/net/proxy"
	"net/http"
	"net/url"
)

type Provider struct{}

type Proxy struct {
	Socks5    proxy.Dialer
	HttpProxy func(*http.Request) (*url.URL, error)
}

func (pp *Proxy) BuildTransport() *http.Transport {
	return ternary.IfLazy(
		pp.HttpProxy != nil,
		func() *http.Transport {
			return &http.Transport{Proxy: pp.HttpProxy}
		},
		func() *http.Transport {
			return &http.Transport{Dial: pp.Socks5.Dial}
		},
	)
}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) (*Proxy, error) {
		pp := &Proxy{}
		if conf.ProxyURL == "" {
			if conf.Socks5Proxy != "" {
				var err error
				pp.Socks5, err = proxy.SOCKS5("tcp", conf.Socks5Proxy, nil, proxy.Direct)
				if err != nil {
					log.Errorf("invalid socks5 proxy url: %s", conf.Socks5Proxy)
					return nil, err
				}
			}

			return pp, nil
		}

		p, err := url.Parse(conf.ProxyURL)
		if err != nil {
			log.Errorf("invalid proxy url: %s", conf.ProxyURL)
			return nil, err
		}

		pp.HttpProxy = http.ProxyURL(p)

		return pp, nil
	})
}

func (Provider) ShouldLoad(conf *config.Config) bool {
	return conf.Socks5Proxy != "" || conf.ProxyURL != ""
}
