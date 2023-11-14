package chat_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/baidu"
	chat2 "github.com/mylxsw/aidea-server/pkg/ai/chat"
	"os"
	"testing"
	"time"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func createBaiduClient() chat2.Chat {
	client := baidu.NewBaiduAI(os.Getenv("BAIDU_WXQF_API_KEY"), os.Getenv("BAIDU_WXQF_SECRET"))
	return chat2.NewBaiduAIChat(client)
}

func TestBaiduAIChat_Chat(t *testing.T) {
	client := createBaiduClient()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := client.Chat(ctx, chat2.Request{
		Model: baidu.ModelChatGLM2_6B_32K,
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

func TestBaiduAIChat_ChatStream(t *testing.T) {
	chatClient := createBaiduClient()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	response, err := chatClient.ChatStream(ctx, chat2.Request{
		Model: baidu.ModelLlama2_70b,
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

	defer fmt.Println("------ END ------")

	for {
		select {
		case res, ok := <-response:
			if !ok {
				return
			}

			if res.ErrorCode != "" {
				log.With(res).Error("error")
				break
			}

			log.With(res).Debug("response")
		case <-ctx.Done():
			return
		}
	}

}
