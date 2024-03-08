package moonshot

import (
	"context"
	oai "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/sashabaranov/go-openai"
)

const ModelMoonshotV1_8K = "moonshot-v1-8k"
const ModelMoonshotV1_32K = "moonshot-v1-32k"
const ModelMoonshotV1_128K = "moonshot-v1-128k"

type Moonshot struct {
	client oai.Client
}

func New(client oai.Client) *Moonshot {
	return &Moonshot{client: client}
}

func (oa *Moonshot) ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan oai.ChatStreamResponse, error) {
	return oa.client.ChatStream(ctx, request)
}

func (oa *Moonshot) Chat(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	return oa.client.CreateChatCompletion(ctx, request)
}
