package chat_test

import (
	"context"
	chat2 "github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"os"
	"testing"
	"time"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	openailib "github.com/sashabaranov/go-openai"
)

func createOpenAIChatClient() chat2.Chat {
	openaiConf := openailib.DefaultConfig(os.Getenv("OPENAI_API_KEY"))
	openaiConf.HTTPClient.Timeout = 300 * time.Second
	openaiConf.APIType = openailib.APITypeOpenAI

	client := openailib.NewClientWithConfig(openaiConf)
	return chat2.NewOpenAIChat(openai.New(nil, []*openailib.Client{client}))
}

func TestOpenAIChat_Chat(t *testing.T) {
	chatClient := createOpenAIChatClient()
	response, err := chatClient.Chat(context.TODO(), chat2.Request{
		Model: "gpt-3.5-turbo",
		Messages: []chat2.Message{
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

	log.With(response).Debug("response")
}

func TestOpenAIChat_ChatStream(t *testing.T) {
	chatClient := createOpenAIChatClient()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	response, err := chatClient.ChatStream(ctx, chat2.Request{
		Model: "gpt-4-vision-preview",
		Messages: []chat2.Message{
			{
				Role: "system",
				MultipartContents: []*chat2.MultipartContent{
					{Text: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃", Type: "text"},
				},
			},
			{
				Role: "user",
				MultipartContents: []*chat2.MultipartContent{
					{Text: "老铁，最近怎么样？", Type: "text"},
				},
			},
		},
		MaxTokens: 4096,
	})

	assert.NoError(t, err)

	for res := range response {
		if res.ErrorCode != "" {
			log.With(res).Error("error")
			break
		}

		log.With(res).Debug("response")
	}
}
