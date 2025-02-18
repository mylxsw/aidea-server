package chat

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/ai/anthropic"
	"github.com/mylxsw/aidea-server/pkg/ai/baichuan"
	"github.com/mylxsw/aidea-server/pkg/ai/baidu"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/aidea-server/pkg/ai/google"
	"github.com/mylxsw/aidea-server/pkg/ai/gpt360"
	"github.com/mylxsw/aidea-server/pkg/ai/moonshot"
	"github.com/mylxsw/aidea-server/pkg/ai/oneapi"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/ai/openrouter"
	"github.com/mylxsw/aidea-server/pkg/ai/sensenova"
	"github.com/mylxsw/aidea-server/pkg/ai/sky"
	"github.com/mylxsw/aidea-server/pkg/ai/tencentai"
	"github.com/mylxsw/aidea-server/pkg/ai/xfyun"
	"github.com/mylxsw/aidea-server/pkg/ai/zhipuai"
	"github.com/mylxsw/aidea-server/pkg/file"
	"github.com/mylxsw/aidea-server/pkg/search"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(resolver infra.Resolver) *AIProvider {
		var aiProvider AIProvider
		resolver.MustAutoWire(&aiProvider)

		return &aiProvider
	})
	binder.MustSingleton(NewAI)
	binder.MustSingleton(func(conf *config.Config, resolver infra.Resolver, svc *service.Service, ai *AI, searcher search.Searcher) Chat {
		return NewChat(conf, resolver, svc, ai, searcher)
	})
}

type AIProvider struct {
	OpenAI     openai.Client          `autowire:"@"`
	Baidu      baidu.BaiduAI          `autowire:"@"`
	Dashscope  *dashscope.DashScope   `autowire:"@"`
	Xfyun      *xfyun.XFYunAI         `autowire:"@"`
	SenseNova  *sensenova.SenseNova   `autowire:"@"`
	Tencent    *tencentai.TencentAI   `autowire:"@"`
	Anthropic  *anthropic.Anthropic   `autowire:"@"`
	Baichuan   *baichuan.BaichuanAI   `autowire:"@"`
	GPT360     *gpt360.GPT360         `autowire:"@"`
	OneAPI     *oneapi.OneAPI         `autowire:"@"`
	Google     *google.GoogleAI       `autowire:"@"`
	OpenRouter *openrouter.OpenRouter `autowire:"@"`
	Sky        *sky.Sky               `autowire:"@"`
	Zhipu      *zhipuai.ZhipuAI       `autowire:"@"`
	Moonshot   *moonshot.Moonshot     `autowire:"@"`
}

type AI struct {
	OpenAI     *OpenAIChat
	Baidu      *BaiduAIChat
	DashScope  *DashScopeChat
	Xfyun      *XFYunChat
	SenseNova  *SenseNovaChat
	Tencent    *TencentAIChat
	Anthropic  *AnthropicChat
	Baichuan   *BaichuanAIChat
	GPT360     *GPT360Chat
	OneAPI     *OneAPIChat
	Google     *GoogleChat
	Openrouter *OpenRouterChat
	Sky        *SkyChat
	Zhipu      *ZhipuChat
	Moonshot   *MoonshotChat
}

func NewAI(
	file *file.File,
	aiProvider *AIProvider,
) *AI {
	return &AI{
		OpenAI:     NewOpenAIChat(aiProvider.OpenAI),
		Baidu:      NewBaiduAIChat(aiProvider.Baidu),
		DashScope:  NewDashScopeChat(aiProvider.Dashscope, file),
		Xfyun:      NewXFYunChat(aiProvider.Xfyun),
		SenseNova:  NewSenseNovaChat(aiProvider.SenseNova),
		Tencent:    NewTencentAIChat(aiProvider.Tencent),
		Anthropic:  NewAnthropicChat(aiProvider.Anthropic),
		Baichuan:   NewBaichuanAIChat(aiProvider.Baichuan),
		GPT360:     NewGPT360Chat(aiProvider.GPT360),
		OneAPI:     NewOneAPIChat(aiProvider.OneAPI),
		Google:     NewGoogleChat(aiProvider.Google),
		Openrouter: NewOpenRouterChat(aiProvider.OpenRouter),
		Sky:        NewSkyChat(aiProvider.Sky),
		Zhipu:      NewZhipuChat(aiProvider.Zhipu),
		Moonshot:   NewMoonshotChat(aiProvider.Moonshot),
	}
}
