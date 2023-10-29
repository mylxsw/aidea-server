package chat_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/internal/ai/baichuan"
	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"os"
	"testing"
)

func TestBaichuanChat_Chat(t *testing.T) {
	client := createBaichuanClient()

	response, err := client.Chat(context.TODO(), chat.Request{
		Model: "百川:" + string(baichuan.ModelBaichuan2_53B),
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

func TestBaichuanChat_ChatStream(t *testing.T) {
	chatClient := createBaichuanClient()
	response, err := chatClient.ChatStream(context.TODO(), chat.Request{
		Model: "百川:" + string(baichuan.ModelBaichuan2_53B),
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

		//log.With(res).Debug("response")
		fmt.Print(res.Text)
	}
}

func createBaichuanClient() chat.Chat {
	client := baichuan.NewBaichuanAI(os.Getenv("BAICHUAN_API_KEY"), os.Getenv("BAICHUAN_SECRET_KEY"))
	return chat.NewBaichuanAIChat(client)
}
