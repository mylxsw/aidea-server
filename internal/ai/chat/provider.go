package chat

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/anthropic"
	"github.com/mylxsw/aidea-server/internal/ai/baichuan"
	"github.com/mylxsw/aidea-server/internal/ai/baidu"
	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/aidea-server/internal/ai/gpt360"
	"github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/aidea-server/internal/ai/sensenova"
	"github.com/mylxsw/aidea-server/internal/ai/tencentai"
	"github.com/mylxsw/aidea-server/internal/ai/xfyun"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(
		conf *config.Config,
		oai *openai.OpenAI,
		bai baidu.BaiduAI,
		ds *dashscope.DashScope,
		xf *xfyun.XFYunAI,
		sn *sensenova.SenseNova,
		tai *tencentai.TencentAI,
		anthai *anthropic.Anthropic,
		baichuanai *baichuan.BaichuanAI,
		g360 *gpt360.GPT360,
	) Chat {
		return NewChat(
			conf,
			NewOpenAIChat(oai),
			NewBaiduAIChat(bai),
			NewDashScopeChat(ds),
			NewXFYunChat(xf),
			NewSenseNovaChat(sn),
			NewTencentAIChat(tai),
			NewAnthropicChat(anthai),
			NewBaichuanAIChat(baichuanai),
			NewGPT360Chat(g360),
		)
	})
}
