package chat

import (
	"context"
	"strings"

	"github.com/mylxsw/aidea-server/internal/ai/baidu"
	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/aidea-server/internal/ai/xfyun"
	"github.com/mylxsw/go-utils/array"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Messages []Message

func (ms Messages) Fix() Messages {
	last := ms[len(ms)-1]
	if last.Role != "user" {
		last = Message{
			Role:    "user",
			Content: "继续",
		}
		ms = append(ms, last)
	}

	systemMsgs := array.Filter(ms, func(m Message, _ int) bool { return m.Role == "system" })
	if len(systemMsgs) > 0 {
		ms = array.Filter(ms, func(m Message, _ int) bool { return m.Role != "system" })
	}

	finalMessages := make([]Message, 0)
	var lastRole string

	for _, m := range array.Reverse(ms) {
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
	Model     string   `json:"model"`
	Messages  Messages `json:"messages"`
	MaxTokens int      `json:"max_tokens,omitempty"`
	N         int      `json:"n,omitempty"` // 复用作为 room_id
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
	Chat(ctx context.Context, req Request) (*Response, error)
	ChatStream(ctx context.Context, req Request) (<-chan Response, error)
}

type ChatImp struct {
	openAI    *OpenAIChat
	baiduAI   *BaiduAIChat
	dashScope *DashScopeChat
	xfyunAI   *XFYunChat
}

func NewChat(openAI *OpenAIChat, baiduAI *BaiduAIChat, dashScope *DashScopeChat, xfyunAI *XFYunChat) Chat {
	return &ChatImp{openAI: openAI, baiduAI: baiduAI, dashScope: dashScope, xfyunAI: xfyunAI}
}

func (ai *ChatImp) selectImp(model string) Chat {
	if strings.HasPrefix(model, "灵积:") {
		return ai.dashScope
	}

	if strings.HasPrefix(model, "文心千帆:") {
		return ai.baiduAI
	}

	if strings.HasPrefix(model, "讯飞星火:") {
		return ai.xfyunAI
	}

	// TODO 根据模型名称判断使用哪个 AI
	switch model {
	case string(baidu.ModelErnieBot), string(baidu.ModelErnieBotTurbo):
		return ai.baiduAI
	case dashscope.ModelQWenV1, dashscope.ModelQWenPlusV1, dashscope.ModelQWen7BV1, dashscope.ModelQWen7BChatV1:
		return ai.dashScope
	case string(xfyun.ModelGeneralV1_5), string(xfyun.ModelGeneralV2):
		return ai.xfyunAI
	}

	return ai.openAI
}

func (ai *ChatImp) Chat(ctx context.Context, req Request) (*Response, error) {
	return ai.selectImp(req.Model).Chat(ctx, req)
}

func (ai *ChatImp) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	return ai.selectImp(req.Model).ChatStream(ctx, req)
}
