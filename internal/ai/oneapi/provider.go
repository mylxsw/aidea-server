package oneapi

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) *OneAPI {
		client := openai.NewOpenAIClient(&openai.Config{
			Enable:        conf.EnableOneAPI,
			OpenAIServers: []string{conf.OneAPIServer},
			OpenAIKeys:    []string{conf.OneAPIKey},
		}, nil)
		return New(client)
	})
}
