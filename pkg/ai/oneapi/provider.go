package oneapi

import (
	"github.com/mylxsw/aidea-server/config"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, trans youdao.Translater) *OneAPI {
		client := openai2.NewOpenAIClient(&openai2.Config{
			Enable:        conf.EnableOneAPI,
			OpenAIServers: []string{conf.OneAPIServer},
			OpenAIKeys:    []string{conf.OneAPIKey},
		}, nil)
		return New(client, trans)
	})
}
