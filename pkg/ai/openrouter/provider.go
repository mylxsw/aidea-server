package openrouter

import (
	"github.com/mylxsw/aidea-server/config"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, resolver infra.Resolver) *OpenRouter {
		var pp *proxy.Proxy
		if conf.SupportProxy() && conf.OpenRouterAutoProxy {
			resolver.MustResolve(func(p *proxy.Proxy) {
				pp = p
			})
		}

		if conf.OpenRouterServer == "" {
			conf.OpenRouterServer = "https://openrouter.ai/api/v1"
		}

		client := openai2.NewOpenAIClient(&openai2.Config{
			Enable:        conf.EnableOpenRouter,
			OpenAIServers: []string{conf.OpenRouterServer},
			OpenAIKeys:    []string{conf.OpenRouterKey},
		}, pp)

		return NewOpenRouter(client)
	})
}
