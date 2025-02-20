package search

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/glacier/infra"

	oai "github.com/mylxsw/aidea-server/pkg/ai/openai"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, assistant *SearchAssistant) Searcher {
		return NewSearcher(conf, assistant)
	})

	binder.MustSingleton(func(conf *config.Config, resolver infra.Resolver) *SearchAssistant {
		if conf.SearchAssistantAPIKey == "" || conf.SearchAssistantAPIBase == "" || conf.SearchAssistantModel == "" {
			return NewSearchAssistant(nil, "")
		}

		var proxyDialer *proxy.Proxy
		if conf.SupportProxy() && ((conf.DalleUsingOpenAISetting && conf.OpenAIAutoProxy) || (!conf.DalleUsingOpenAISetting && conf.OpenAIDalleAutoProxy)) {
			resolver.MustResolve(func(pp *proxy.Proxy) {
				proxyDialer = pp
			})
		}

		oaiconf := oai.Config{
			Enable:        true,
			OpenAIKeys:    []string{conf.SearchAssistantAPIKey},
			OpenAIServers: []string{conf.SearchAssistantAPIBase},
		}
		client := oai.NewOpenAIClient(&oaiconf, proxyDialer)

		return NewSearchAssistant(client, conf.SearchAssistantModel)
	})
}
