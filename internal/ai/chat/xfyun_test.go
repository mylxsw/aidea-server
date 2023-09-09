package chat_test

import (
	"context"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/aidea-server/internal/ai/xfyun"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestXFYunChat_Chat(t *testing.T) {
	client := createXFClient()

	response, err := client.Chat(context.TODO(), chat.Request{
		Model: "讯飞星火:" + string(xfyun.ModelGeneralV2),
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

func TestXFYunChat_ChatStream(t *testing.T) {
	chatClient := createXFClient()
	response, err := chatClient.ChatStream(context.TODO(), chat.Request{
		Model: "讯飞星火:" + string(xfyun.ModelGeneralV2),
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

func createXFClient() chat.Chat {
	client := xfyun.New(os.Getenv("XFYUN_APPID"), os.Getenv("XFYUN_API_KEY"), os.Getenv("XFYUN_API_SECRET"))

	return chat.NewXFYunChat(client)
}
