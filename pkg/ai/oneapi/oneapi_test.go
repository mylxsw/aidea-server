package oneapi_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/oneapi"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"github.com/sashabaranov/go-openai"
	"os"
	"testing"
	"time"
)

func TestOneAPI_ChatStream(t *testing.T) {
	client := createClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, err := client.ChatStream(ctx, openai.ChatCompletionRequest{
		Model: "PaLM-2",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃",
			},
			{
				Role:    "user",
				Content: "老铁，最近怎么样？",
			},
		},
	})

	assert.NoError(t, err)

	for res := range response {
		log.With(res).Debugf("-> %s", res.ChatResponse.Choices[0].Delta.Content)
	}
}

func createClient() *oneapi.OneAPI {
	conf := &openai2.Config{
		Enable:        true,
		OpenAIServers: []string{os.Getenv("ONEAPI_URL")},
		OpenAIKeys:    []string{os.Getenv("ONEAPI_KEY")},
	}
	return oneapi.New(openai2.NewOpenAIClient(conf, nil), nil)
}

func TestOneAPI_Chat(t *testing.T) {
	client := createClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, err := client.Chat(ctx, openai.ChatCompletionRequest{
		Model: "PaLM-2",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃",
			},
			{
				Role:    "user",
				Content: "老铁，最近怎么样？",
			},
		},
	})

	assert.NoError(t, err)

	log.With(response).Debugf("response")
}
