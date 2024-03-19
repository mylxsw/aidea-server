package chat

import (
	"context"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/google"
	"github.com/mylxsw/aidea-server/pkg/ai/oneapi"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/ai/openrouter"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"strings"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/go-utils/array"
)

var (
	ErrContextExceedLimit = errors.New("上下文长度超过最大限制")
	ErrContentFilter      = errors.New("请求或响应内容包含敏感词")
)

type Message struct {
	Role              string              `json:"role"`
	Content           string              `json:"content"`
	MultipartContents []*MultipartContent `json:"multipart_content,omitempty"`
}

type MultipartContent struct {
	// Type 对于 OpenAI 来说， type 可选值为 image_url/text
	Type     string    `json:"type"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
	Text     string    `json:"text,omitempty"`
}

type ImageURL struct {
	// URL Either a URL of the image or the base64 encoded image data.
	URL string `json:"url,omitempty"`
	// Detail Specifies the detail level of the image
	// Three options, low, high, or auto, you have control over how the model processes the image and generates its textual understanding.
	// By default, the model will use the auto setting which will look at the image input size and decide if it should use the low or high setting
	//
	// - `low` will disable the “high res” model. The model will receive a low-res 512px x 512px version of the image,
	//   and represent the image with a budget of 65 tokens. This allows the API to return faster responses and consume
	//   fewer input tokens for use cases that do not require high detail.
	//
	// - `high` will enable “high res” mode, which first allows the model to see the low res image and
	//   then creates detailed crops of input images as 512px squares based on the input image size.
	//   Each of the detailed crops uses twice the token budget (65 tokens) for a total of 129 tokens.
	Detail string `json:"detail,omitempty"`
}

type Messages []Message

func (ms Messages) HasImage() bool {
	for _, msg := range ms {
		for _, part := range msg.MultipartContents {
			if part.ImageURL != nil && part.ImageURL.URL != "" {
				return true
			}
		}
	}

	return false
}

func (ms Messages) Fix() Messages {
	msgs := ms
	// 如果最后一条消息不是用户消息，则补充一条用户消息
	last := msgs[len(msgs)-1]
	if last.Role != "user" {
		last = Message{
			Role:    "user",
			Content: "继续",
		}
		msgs = append(msgs, last)
	}

	// 过滤掉 system 消息，因为 system 消息需要在每次对话中保留，不受上下文长度限制
	systemMsgs := array.Filter(msgs, func(m Message, _ int) bool { return m.Role == "system" })
	if len(systemMsgs) > 0 {
		msgs = array.Filter(msgs, func(m Message, _ int) bool { return m.Role != "system" })
	}

	finalMessages := make([]Message, 0)
	var lastRole string

	for _, m := range array.Reverse(msgs) {
		if m.Role == lastRole {
			continue
		}

		lastRole = m.Role
		finalMessages = append(finalMessages, m)
	}

	if len(finalMessages)%2 == 0 {
		finalMessages = finalMessages[:len(finalMessages)-1]
	}

	return append(systemMsgs, array.Reverse(finalMessages)...)
}

// Request represents a request structure for chat completion API.
type Request struct {
	Stream    bool     `json:"stream,omitempty"`
	Model     string   `json:"model"`
	Messages  Messages `json:"messages"`
	MaxTokens int      `json:"max_tokens,omitempty"`
	N         int      `json:"n,omitempty"` // 复用作为 room_id

	// 业务定制字段
	RoomID    int64 `json:"-"`
	WebSocket bool  `json:"-"`
}

func (req Request) assembleMessage() string {
	var msgs []string
	for _, msg := range req.Messages {
		msgs = append(msgs, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}

	return strings.Join(msgs, "\n\n")
}

func (req Request) Init() Request {
	// 去掉模型名称前缀
	modelSegs := strings.Split(req.Model, ":")
	if len(modelSegs) > 1 {
		modelSegs = modelSegs[1:]
	}

	req.Model = strings.Join(modelSegs, ":")

	// 获取 room id
	// 这里复用了参数 N
	req.RoomID = int64(req.N)
	if req.N != 0 {
		req.N = 0
	}

	// 过滤掉内容为空的 message
	req.Messages = array.Filter(req.Messages, func(item Message, _ int) bool { return strings.TrimSpace(item.Content) != "" })

	// TODO 临时方案，对于 Google Gemini Pro Vision 模型，有以下特性:
	// 1. 不支持多轮对话
	// 2. 请求中必须包含图片
	if req.Model == google.ModelGeminiProVision && len(req.Messages) > 1 {
		// 只保留最后一条消息
		req.Messages = req.Messages[len(req.Messages)-1:]
	}

	return req
}

// Fix 修复请求内容，注意：上下文长度修复后，最终的上下文数量不包含 system 消息和用户最后一条消息
func (req Request) Fix(chat Chat, maxContextLength int64, maxTokenCount int) (*Request, int64, error) {
	// 自动缩减上下文长度至满足模型要求的最大长度，尽可能避免出现超过模型上下文长度的问题
	systemMessages := array.Filter(req.Messages, func(item Message, _ int) bool { return item.Role == "system" })
	systemMessageLen, _ := MessageTokenCount(systemMessages, req.Model)

	// 模型允许的 Tokens 数量和请求参数指定的 Tokens 数量，取最小值
	modelTokenLimit := chat.MaxContextLength(req.Model) - systemMessageLen
	if modelTokenLimit < maxTokenCount {
		maxTokenCount = modelTokenLimit
	}

	messages, inputTokens, err := ReduceMessageContext(
		ReduceMessageContextUpToContextWindow(
			array.Filter(req.Messages, func(item Message, _ int) bool { return item.Role != "system" }),
			int(maxContextLength),
		),
		req.Model,
		maxTokenCount,
	)
	if err != nil {
		return nil, 0, errors.New("超过模型最大允许的上下文长度限制，请尝试“新对话”或缩短输入内容长度")
	}

	req.Messages = array.Map(append(systemMessages, messages...), func(item Message, _ int) Message {
		if len(item.MultipartContents) > 0 {
			item.MultipartContents = array.Map(item.MultipartContents, func(part *MultipartContent, _ int) *MultipartContent {
				if part.ImageURL != nil && part.ImageURL.URL != "" && part.ImageURL.Detail == "" {
					part.ImageURL.Detail = "low"
				}

				return part
			})
		}
		return item
	})

	return &req, int64(inputTokens), nil
}

func (req Request) ResolveCalFeeModel(conf *config.Config) string {
	return req.Model
}

type Response struct {
	Error        string `json:"error,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	Text         string `json:"text,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

type Chat interface {
	// Chat 以请求-响应的方式进行对话
	Chat(ctx context.Context, req Request) (*Response, error)
	// ChatStream 以流的方式进行对话
	ChatStream(ctx context.Context, req Request) (<-chan Response, error)
	// MaxContextLength 获取模型的最大上下文长度
	MaxContextLength(model string) int
}

type Imp struct {
	ai       *AI
	svc      *service.Service
	proxy    *proxy.Proxy
	resolver infra.Resolver
}

func NewChat(conf *config.Config, resolver infra.Resolver, svc *service.Service, ai *AI) Chat {
	var proxyDialer *proxy.Proxy
	if conf.SupportProxy() {
		resolver.MustResolve(func(pp *proxy.Proxy) {
			proxyDialer = pp
		})
	}

	return &Imp{ai: ai, svc: svc, proxy: proxyDialer, resolver: resolver}
}

func (ai *Imp) queryModel(modelId string) repo.Model {
	pro := ai.svc.Chat.Model(context.Background(), modelId)
	if pro == nil {
		pro = &repo.Model{
			Providers: []repo.ModelProvider{
				{Name: service.ProviderOpenAI},
			},
			Models: model.Models{
				ModelId: modelId,
			},
			Meta: repo.ModelMeta{
				Restricted: true,
				MaxContext: 4000,
			},
		}
	}

	return *pro
}

// selectImp 选择合适的 AI 服务提供商
//
// 并不是所有类型的渠道都支持动态配置（根据数据库 channels 中的配置创建客户端），目前只有 openai/oneapi/openrouter 支持
// 首先 根据 Channel ID 选择对应的 AI 服务提供商，如果 Channel ID 不存在或者对应的 AI 服务提供商不支持，则根据 Model ID 选择对应的 AI 服务提供商
// 如果 Model ID 也不存在或者对应的 AI 服务提供商不支持，则使用 OpenAI 作为默认的 AI 服务提供商
func (ai *Imp) selectImp(provider repo.ModelProvider) Chat {
	if provider.ID > 0 {
		ch, err := ai.svc.Chat.Channel(context.Background(), provider.ID)
		if err != nil {
			log.F(log.M{"provider": provider}).Errorf("get channel %d failed: %v", provider.ID, err)
		} else {
			switch ch.Type {
			case service.ProviderOpenAI:
				return ai.createOpenAIClient(ch)
			case service.ProviderOneAPI:
				return ai.createOneAPIClient(ch)
			case service.ProviderOpenRouter:
				return ai.createOpenRouterClient(ch)
			default:
				if ret := ai.selectProvider(ch.Type); ret != nil {
					return ret
				}
			}
		}
	}

	if ret := ai.selectProvider(provider.Name); ret != nil {
		return ret
	}

	log.Errorf("unsupported provider: %s, using openai instead", provider.Name)

	return ai.ai.OpenAI
}

func (ai *Imp) selectProvider(name string) Chat {
	switch name {
	case service.ProviderOpenAI:
		return ai.ai.OpenAI
	case service.ProviderXunFei:
		return ai.ai.Xfyun
	case service.ProviderWenXin:
		return ai.ai.Baidu
	case service.ProviderDashscope:
		return ai.ai.DashScope
	case service.ProviderSenseNova:
		return ai.ai.SenseNova
	case service.ProviderTencent:
		return ai.ai.Tencent
	case service.ProviderBaiChuan:
		return ai.ai.Baichuan
	case service.Provider360:
		return ai.ai.GPT360
	case service.ProviderOneAPI:
		return ai.ai.OneAPI
	case service.ProviderOpenRouter:
		return ai.ai.Openrouter
	case service.ProviderSky:
		return ai.ai.Sky
	case service.ProviderZhipu:
		return ai.ai.Zhipu
	case service.ProviderMoonshot:
		return ai.ai.Moonshot
	case service.ProviderGoogle:
		return ai.ai.Google
	case service.ProviderAnthropic:
		return ai.ai.Anthropic
	default:
	}

	return nil
}

func (ai *Imp) Chat(ctx context.Context, req Request) (*Response, error) {
	mod := ai.queryModel(req.Model)
	pro := mod.SelectProvider()

	if pro.Prompt != "" {
		systemPrompts := array.Filter(req.Messages, func(item Message, _ int) bool { return item.Role == "system" })
		chatMessages := array.Filter(req.Messages, func(item Message, _ int) bool { return item.Role != "system" })

		if len(systemPrompts) > 0 {
			systemPrompts[0].Content = pro.Prompt + "\n" + systemPrompts[0].Content
			systemPrompts = Messages{systemPrompts[0]}
		}

		req.Messages = append(systemPrompts, chatMessages...)
	}

	if pro.ModelRewrite != "" {
		req.Model = pro.ModelRewrite
	}

	return ai.selectImp(pro).Chat(ctx, req)
}

func (ai *Imp) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	// TODO 这里是临时解决方案
	// 使用微软的 Azure OpenAI 接口时，聊天内容只有“继续”两个字时，会触发风控，导致无法继续对话
	req.Messages = array.Map(req.Messages, func(item Message, _ int) Message {
		content := strings.TrimSpace(item.Content)
		if content == "继续" {
			item.Content = "请接着说"
		}

		return item
	})

	mod := ai.queryModel(req.Model)
	pro := mod.SelectProvider()

	if pro.Prompt != "" {
		systemPrompts := array.Filter(req.Messages, func(item Message, _ int) bool { return item.Role == "system" })
		chatMessages := array.Filter(req.Messages, func(item Message, _ int) bool { return item.Role != "system" })

		if len(systemPrompts) > 0 {
			systemPrompts[0].Content = pro.Prompt + "\n" + systemPrompts[0].Content
			systemPrompts = Messages{systemPrompts[0]}
		}

		req.Messages = append(systemPrompts, chatMessages...)
	}

	if pro.ModelRewrite != "" {
		req.Model = pro.ModelRewrite
	}

	return ai.selectImp(pro).ChatStream(ctx, req)
}

func (ai *Imp) MaxContextLength(model string) int {
	mod := ai.queryModel(model)
	if mod.Meta.MaxContext > 0 {
		return mod.Meta.MaxContext
	}

	return ai.selectImp(mod.SelectProvider()).MaxContextLength(model)
}

// createOpenAIClient 创建一个 OpenAI Client
func (ai *Imp) createOpenAIClient(ch *repo.Channel) Chat {
	conf := openai.Config{
		Enable:        true,
		OpenAIServers: []string{ch.Server},
		OpenAIKeys:    []string{ch.Secret},
		AutoProxy:     ch.Meta.UsingProxy,
	}

	if ch.Meta.OpenAIAzure {
		conf.OpenAIAzure = true
		conf.OpenAIAPIVersion = ch.Meta.OpenAIAzureAPIVersion
	}

	return NewOpenAIChat(openai.NewOpenAIClient(&conf, ai.proxy))
}

// createOneAPIClient 创建一个 OneAPI Client
func (ai *Imp) createOneAPIClient(ch *repo.Channel) Chat {
	conf := openai.Config{
		Enable:        true,
		OpenAIServers: []string{ch.Server},
		OpenAIKeys:    []string{ch.Secret},
		AutoProxy:     ch.Meta.UsingProxy,
	}

	var trans youdao.Translater
	_ = ai.resolver.Resolve(func(t youdao.Translater) {
		trans = t
	})

	return NewOneAPIChat(oneapi.New(openai.NewOpenAIClient(&conf, ai.proxy), trans))
}

// createOpenRouterClient 创建一个 OpenRouter Client
func (ai *Imp) createOpenRouterClient(ch *repo.Channel) Chat {
	if ch.Server == "" {
		ch.Server = "https://openrouter.ai/api/v1"
	}

	conf := openai.Config{
		Enable:        true,
		OpenAIServers: []string{ch.Server},
		OpenAIKeys:    []string{ch.Secret},
		AutoProxy:     ch.Meta.UsingProxy,
	}

	return NewOpenRouterChat(openrouter.NewOpenRouter(openai.NewOpenAIClient(&conf, ai.proxy)))
}
