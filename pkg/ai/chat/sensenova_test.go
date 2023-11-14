package chat_test

import (
	"context"
	"fmt"
	chat2 "github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/ai/sensenova"
	"os"
	"testing"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestSenseNovaChat_Chat(t *testing.T) {
	client := createSNClient()

	response, err := client.Chat(context.TODO(), chat2.Request{
		Model: "商汤日日新:" + string(sensenova.ModelNovaPtcXLV1),
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

func TestSenseNovaChat_ChatStream(t *testing.T) {
	chatClient := createSNClient()
	response, err := chatClient.ChatStream(context.TODO(), chat2.Request{
		Model: "商汤日日新:" + string(sensenova.ModelNovaPtcXLV1),
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

		//log.With(res).Debug("response")
		fmt.Print(res.Text)
	}
}

func createSNClient() chat2.Chat {
	client := sensenova.New(os.Getenv("SENSENOVA_KEY_ID"), os.Getenv("SENSENOVA_KEY_SECRET"))
	return chat2.NewSenseNovaChat(client)
}
