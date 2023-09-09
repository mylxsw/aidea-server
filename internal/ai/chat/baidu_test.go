package chat_test

import (
	"context"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/internal/ai/baidu"
	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func createBaiduClient() chat.Chat {
	client := baidu.NewBaiduAI(os.Getenv("BAIDU_WXQF_API_KEY"), os.Getenv("BAIDU_WXQF_SECRET"))
	return chat.NewBaiduAIChat(client)
}

func TestBaiduAIChat_Chat(t *testing.T) {
	client := createBaiduClient()

	response, err := client.Chat(context.TODO(), chat.Request{
		Model: baidu.ModelErnieBotTurbo,
		Messages: []chat.Message{
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

func TestBaiduAIChat_ChatStream(t *testing.T) {
	chatClient := createBaiduClient()
	response, err := chatClient.ChatStream(context.TODO(), chat.Request{
		Model: baidu.ModelErnieBotTurbo,
		Messages: []chat.Message{
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
		if res.ErrorCode != "" {
			log.With(res).Error("error")
			break
		}

		log.With(res).Debug("response")
	}
}
