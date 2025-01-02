package openrouter

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/glacier/infra"
	"net/http"
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

		client := openai.NewOpenAIClient(&openai.Config{
			Enable:        conf.EnableOpenRouter,
			OpenAIServers: []string{conf.OpenRouterServer},
			OpenAIKeys:    []string{conf.OpenRouterKey},
			Header: http.Header{
				"HTTP-Referer": []string{"https://web.aicode.cc"},
				"X-Title":      []string{"AIdea"},
			},
		}, pp)

		return NewOpenRouter(client)
	})
}
