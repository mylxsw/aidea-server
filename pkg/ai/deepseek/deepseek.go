package deepseek

import (
	"context"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/sashabaranov/go-openai"
)

type DeepSeek struct {
	client openai2.Client
}

func NewDeepSeek(client openai2.Client) *DeepSeek {
	return &DeepSeek{client: client}
}

func (oa *DeepSeek) ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan openai2.ChatStreamResponse, error) {
	return oa.client.ChatStream(ctx, request)
}

func (oa *DeepSeek) Chat(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	return oa.client.CreateChatCompletion(ctx, request)
}
