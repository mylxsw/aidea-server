package anthropic

import (
	"net/http"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
	"golang.org/x/net/proxy"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, resolver infra.Resolver) *Anthropic {
		client := &http.Client{}
		if conf.Socks5Proxy != "" && conf.AnthropicAutoProxy {
			resolver.MustResolve(func(dialer proxy.Dialer) {
				client.Transport = &http.Transport{Dial: dialer.Dial}
			})
		}

		return New(conf.AnthropicServer, conf.AnthropicAPIKey, client)
	})
}
