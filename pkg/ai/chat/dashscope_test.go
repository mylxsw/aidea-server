package chat_test

import (
	"context"
	chat2 "github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"os"
	"testing"
	"time"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func createDashScopeChatClient() chat2.Chat {
	return chat2.NewDashScopeChat(dashscope.New(os.Getenv("ALI_LINGJI_API_KEY")))
}

func TestDashscopeChat_Chat(t *testing.T) {
	client := createDashScopeChatClient()

	response, err := client.Chat(context.TODO(), chat2.Request{
		Model: dashscope.ModelQWenV1,
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

func TestDashscopeChat_ChatStream(t *testing.T) {
	chatClient := createDashScopeChatClient()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	response, err := chatClient.ChatStream(ctx, chat2.Request{
		Model: dashscope.ModelQWenV1,
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

	for res := range response {
		if res.ErrorCode != "" {
			log.With(res).Error("error")
			break
		}

		log.With(res).Debug("response")
	}
}
