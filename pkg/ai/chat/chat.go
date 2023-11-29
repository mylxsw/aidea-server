package chat

import (
	"context"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/anthropic"
	"github.com/mylxsw/aidea-server/pkg/ai/baichuan"
	"github.com/mylxsw/aidea-server/pkg/ai/baidu"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/aidea-server/pkg/ai/gpt360"
	"github.com/mylxsw/aidea-server/pkg/ai/sensenova"
	"github.com/mylxsw/aidea-server/pkg/ai/tencentai"
	"github.com/mylxsw/aidea-server/pkg/ai/xfyun"
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

	model := strings.Join(modelSegs, ":")
	req.Model = model

	// 获取 room id
	// 这里复用了参数 N
	req.RoomID = int64(req.N)
	if req.N != 0 {
		req.N = 0
	}

	// 过滤掉内容为空的 message
	req.Messages = array.Filter(req.Messages, func(item Message, _ int) bool { return strings.TrimSpace(item.Content) != "" })
	return req
}

// Fix 修复请求内容，注意：上下文长度修复后，最终的上下文数量不包含 system 消息和用户最后一条消息
func (req Request) Fix(chat Chat, maxContextLength int64) (*Request, int64, error) {
	// 自动缩减上下文长度至满足模型要求的最大长度，尽可能避免出现超过模型上下文长度的问题
	systemMessages := array.Filter(req.Messages, func(item Message, _ int) bool { return item.Role == "system" })
	systemMessageLen, _ := MessageTokenCount(systemMessages, req.Model)

	messages, inputTokens, err := ReduceMessageContext(
		ReduceMessageContextUpToContextWindow(
			array.Filter(req.Messages, func(item Message, _ int) bool { return item.Role != "system" }),
			int(maxContextLength),
		),
		req.Model,
		chat.MaxContextLength(req.Model)-systemMessageLen,
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

	// TODO 这里是临时解决方案
	req.MaxTokens = 4000

	return &req, int64(inputTokens), nil
}

func (req Request) ResolveCalFeeModel(conf *config.Config) string {
	if req.Model == "nanxian" {
		return conf.VirtualModel.NanxianRel
	}

	if req.Model == "beichou" {
		return conf.VirtualModel.BeichouRel
	}

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
	openAI      *OpenAIChat
	baiduAI     *BaiduAIChat
	dashScope   *DashScopeChat
	xfyunAI     *XFYunChat
	snAI        *SenseNovaChat
	tencentAI   *TencentAIChat
	anthropicAI *AnthropicChat
	baichuanAI  *BaichuanAIChat
	g360        *GPT360Chat
	virtual     *VirtualChat
	one         *OneAPIChat
}

func NewChat(
	conf *config.Config,
	openAI *OpenAIChat,
	baiduAI *BaiduAIChat,
	dashScope *DashScopeChat,
	xfyunAI *XFYunChat,
	sn *SenseNovaChat,
	tencentAI *TencentAIChat,
	anthropicAI *AnthropicChat,
	baichuanAI *BaichuanAIChat,
	g360 *GPT360Chat,
	one *OneAPIChat,
) Chat {
	var virtualImpl Chat
	switch strings.ToLower(conf.VirtualModel.Implementation) {
	case "openai":
		virtualImpl = openAI
	case "baidu", "文心千帆":
		virtualImpl = baiduAI
	case "dashscope", "灵积":
		virtualImpl = dashScope
	case "xfyun", "讯飞星火":
		virtualImpl = xfyunAI
	case "sense_nova", "商汤日日新":
		virtualImpl = sn
	case "tencent", "腾讯":
		virtualImpl = tencentAI
	case "anthropic":
		virtualImpl = anthropicAI
	case "baichuan", "百川":
		virtualImpl = baichuanAI
	case "gpt360", "360智脑":
		virtualImpl = g360
	case "chatglm_turbo", "chatglm_pro", "chatglm_lite", "chatglm_std", "PaLM-2":
		virtualImpl = one
	default:
		virtualImpl = openAI
	}

	return &Imp{
		openAI:      openAI,
		baiduAI:     baiduAI,
		dashScope:   dashScope,
		xfyunAI:     xfyunAI,
		snAI:        sn,
		tencentAI:   tencentAI,
		anthropicAI: anthropicAI,
		baichuanAI:  baichuanAI,
		g360:        g360,
		virtual:     NewVirtualChat(virtualImpl, conf.VirtualModel),
		one:         one,
	}
}

func (ai *Imp) selectImp(model string) Chat {
	if strings.HasPrefix(model, "灵积:") {
		return ai.dashScope
	}

	if strings.HasPrefix(model, "文心千帆:") {
		return ai.baiduAI
	}

	if strings.HasPrefix(model, "讯飞星火:") {
		return ai.xfyunAI
	}

	if strings.HasPrefix(model, "商汤日日新:") {
		return ai.snAI
	}

	if strings.HasPrefix(model, "腾讯:") {
		return ai.tencentAI
	}

	if strings.HasPrefix(model, "Anthropic:") {
		return ai.anthropicAI
	}

	if strings.HasPrefix(model, "百川:") {
		return ai.baichuanAI
	}

	if strings.HasPrefix(model, "virtual:") {
		return ai.virtual
	}

	if strings.HasPrefix(model, "360智脑:") {
		return ai.g360
	}

	if strings.HasPrefix(model, "oneapi:") {
		return ai.one
	}

	// TODO 根据模型名称判断使用哪个 AI
	switch model {
	case string(baidu.ModelErnieBot),
		baidu.ModelErnieBotTurbo,
		baidu.ModelErnieBot4,
		baidu.ModelAquilaChat7B,
		baidu.ModelChatGLM2_6B_32K,
		baidu.ModelBloomz7B,
		baidu.ModelLlama2_13b,
		baidu.ModelLlama2_7b_CN,
		baidu.ModelLlama2_70b:
		// 百度文心千帆
		return ai.baiduAI
	case dashscope.ModelQWenV1, dashscope.ModelQWenPlusV1,
		dashscope.ModelQWen7BV1, dashscope.ModelQWen7BChatV1,
		dashscope.ModelQWenTurbo, dashscope.ModelQWenPlus, dashscope.ModelBaiChuan7BChatV1,
		dashscope.ModelQWen7BChat, dashscope.ModelQWen14BChat:
		// 阿里灵积平台
		return ai.dashScope
	case string(xfyun.ModelGeneralV1_5), string(xfyun.ModelGeneralV2), string(xfyun.ModelGeneralV3):
		// 讯飞星火
		return ai.xfyunAI
	case string(sensenova.ModelNovaPtcXLV1), string(sensenova.ModelNovaPtcXSV1):
		// 商汤日日新
		return ai.snAI
	case tencentai.ModelHyllm:
		// 腾讯混元大模型
		return ai.tencentAI
	case string(anthropic.ModelClaude2), string(anthropic.ModelClaudeInstant):
		// Anthropic
		return ai.anthropicAI
	case baichuan.ModelBaichuan2_53B:
		// 百川
		return ai.baichuanAI
	case gpt360.Model360GPT_S2_V9:
		// 360智脑
		return ai.g360
	case ModelNanXian, ModelBeiChou:
		// 虚拟模型
		return ai.virtual
	case "chatglm_turbo", "chatglm_pro", "chatglm_lite", "chatglm_std", "PaLM-2":
		// oneapi
		return ai.one
	}

	return ai.openAI
}

func (ai *Imp) Chat(ctx context.Context, req Request) (*Response, error) {
	return ai.selectImp(req.Model).Chat(ctx, req)
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

	return ai.selectImp(req.Model).ChatStream(ctx, req)
}

func (ai *Imp) MaxContextLength(model string) int {
	return ai.selectImp(model).MaxContextLength(model)
}
