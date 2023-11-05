package oneapi

import (
	"context"
	oai "github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/sashabaranov/go-openai"
)

type OneAPI struct {
	client oai.Client
}

func New(client oai.Client) *OneAPI {
	return &OneAPI{client: client}
}

func (oa *OneAPI) ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan oai.ChatStreamResponse, error) {
	return oa.client.ChatStream(ctx, request)
}

func (oa *OneAPI) Chat(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	return oa.client.CreateChatCompletion(ctx, request)
}
