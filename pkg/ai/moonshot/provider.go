package moonshot

import (
	"github.com/mylxsw/aidea-server/config"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, trans youdao.Translater) *Moonshot {
		return New(openai2.NewOpenAIClient(&openai2.Config{
			Enable:        conf.EnableMoonshot,
			OpenAIServers: []string{"https://api.moonshot.cn/v1"},
			OpenAIKeys:    []string{conf.MoonshotAPIKey},
		}, nil))
	})
}
