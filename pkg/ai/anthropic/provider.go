package anthropic

import (
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"net/http"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, resolver infra.Resolver) *Anthropic {
		client := &http.Client{}
		if conf.SupportProxy() && conf.AnthropicAutoProxy {
			resolver.MustResolve(func(pp *proxy.Proxy) {
				client.Transport = pp.BuildTransport()
			})
		}

		return New(conf.AnthropicServer, conf.AnthropicAPIKey, client)
	})
}
