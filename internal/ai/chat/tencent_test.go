package chat_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/aidea-server/internal/ai/tencentai"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"github.com/mylxsw/go-utils/must"
)

func createTencentClient() chat.Chat {
	client := tencentai.New(
		must.Must(strconv.Atoi(os.Getenv("TENCENTCLOUD_APPID"))),
		os.Getenv("TENCENTCLOUD_SECRET_ID"),
		os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	)
	return chat.NewTencentAIChat(client)
}

func TestTencentAIChat_Chat(t *testing.T) {
	client := createTencentClient()

	response, err := client.Chat(context.TODO(), chat.Request{
		Model: tencentai.ModelHyllm,
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

func TestTencentAIChat_ChatStream(t *testing.T) {
	chatClient := createTencentClient()
	response, err := chatClient.ChatStream(context.TODO(), chat.Request{
		Model: tencentai.ModelHyllm,
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
