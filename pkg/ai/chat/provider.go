package chat

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/ai/anthropic"
	"github.com/mylxsw/aidea-server/pkg/ai/baichuan"
	"github.com/mylxsw/aidea-server/pkg/ai/baidu"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/aidea-server/pkg/ai/gpt360"
	"github.com/mylxsw/aidea-server/pkg/ai/oneapi"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/ai/sensenova"
	"github.com/mylxsw/aidea-server/pkg/ai/tencentai"
	"github.com/mylxsw/aidea-server/pkg/ai/xfyun"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(
		conf *config.Config,
		oai openai.Client,
		bai baidu.BaiduAI,
		ds *dashscope.DashScope,
		xf *xfyun.XFYunAI,
		sn *sensenova.SenseNova,
		tai *tencentai.TencentAI,
		anthai *anthropic.Anthropic,
		baichuanai *baichuan.BaichuanAI,
		g360 *gpt360.GPT360,
		one *oneapi.OneAPI,
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
			NewOneAPIChat(one),
		)
	})
}
