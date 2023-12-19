package openrouter_test

import (
	"context"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/ai/openrouter"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"github.com/sashabaranov/go-openai"
	"os"
	"testing"
	"time"
)

func TestOpenRouter_ChatStream(t *testing.T) {
	client := createClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, err := client.ChatStream(ctx, openai.ChatCompletionRequest{
		Model: "01-ai/yi-34b-chat",
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
		MaxTokens: 500,
	})

	assert.NoError(t, err)

	var finalText string
	for res := range response {
		if res.Code != "" {
			log.With(res).Errorf("-> %s", res.ErrorMessage)
			continue
		}

		log.Debugf("-> %s", res.ChatResponse.Choices[0].Delta.Content)
		finalText += res.ChatResponse.Choices[0].Delta.Content
	}

	log.Debugf("final text: %s", finalText)
}

func createClient() *openrouter.OpenRouter {
	conf := &openai2.Config{
		Enable:        true,
		OpenAIServers: []string{"https://openrouter.ai/api/v1"},
		OpenAIKeys:    []string{os.Getenv("OPEN_ROUTER_API_KEY")},
	}
	return openrouter.NewOpenRouter(openai2.NewOpenAIClient(conf, nil))
}

func TestOpenRouter_Chat(t *testing.T) {
	client := createClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, err := client.Chat(ctx, openai.ChatCompletionRequest{
		Model: "01-ai/yi-34b-chat",
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
