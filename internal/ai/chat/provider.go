package chat

import (
	"github.com/mylxsw/aidea-server/internal/ai/baidu"
	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/aidea-server/internal/ai/xfyun"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(oai *openai.OpenAI, bai *baidu.BaiduAI, ds *dashscope.DashScope, xf *xfyun.XFYunAI) Chat {
		return NewChat(
			NewOpenAIChat(oai),
			NewBaiduAIChat(bai),
			NewDashScopeChat(ds),
			NewXFYunChat(xf),
		)
	})
}
