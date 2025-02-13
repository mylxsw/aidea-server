package openrouter

import (
	"context"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/sashabaranov/go-openai"
)

type OpenRouter struct {
	client openai2.Client
}

func NewOpenRouter(client openai2.Client) *OpenRouter {
	return &OpenRouter{client: client}
}

func (oa *OpenRouter) ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan openai2.ChatStreamResponse, error) {
	request.IncludeReasoning = true
	return oa.client.ChatStream(ctx, request)
}

func (oa *OpenRouter) Chat(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	request.IncludeReasoning = true
	return oa.client.CreateChatCompletion(ctx, request)
}
