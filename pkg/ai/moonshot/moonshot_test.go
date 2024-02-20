package moonshot_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/moonshot"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"github.com/sashabaranov/go-openai"
	"testing"
	"time"
)

func TestMoonshotAPI_ChatStream(t *testing.T) {
	client := createClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, err := client.ChatStream(ctx, openai.ChatCompletionRequest{
		Model: moonshot.ModelMoonshotV1_8K,
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

func createClient() *moonshot.Moonshot {
	conf := &openai2.Config{
		Enable:        true,
		OpenAIServers: []string{"https://api.moonshot.cn/v1"},
		OpenAIKeys:    []string{"sk-X6HJQ0EEYIjbFZreEsuh2fEKxHzM1iqgxbwYOnoXKrSOK4Iz"},
	}
	return moonshot.New(openai2.NewOpenAIClient(conf, nil))
}

func TestMoonshotAPI_Chat(t *testing.T) {
	client := createClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, err := client.Chat(ctx, openai.ChatCompletionRequest{
		Model: moonshot.ModelMoonshotV1_32K,
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
