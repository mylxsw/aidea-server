package dashscope_test

import (
	"context"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func createClient() *dashscope.DashScope {
	return dashscope.New(os.Getenv("ALI_LINGJI_API_KEY"))
}

func TestDashScope_Chat(t *testing.T) {
	client := createClient()
	resp, err := client.Chat(context.TODO(), dashscope.ChatRequest{
		Model: dashscope.ModelQWenPlus,
		Input: dashscope.ChatInput{
			Prompt: "鲁迅为什么要暴打周树人呢",
		},
	})
	assert.NoError(t, err)

	log.With(resp).Debug("resp")
}
func TestDashScope_Chat_VL(t *testing.T) {
	client := createClient()
	resp, err := client.Chat(context.TODO(), dashscope.ChatRequest{
		Model: dashscope.ModelQWenVLPlus,
		Input: dashscope.ChatInput{
			Messages: []dashscope.Message{
				{
					Role: "system",
					Content: []dashscope.MessageContent{
						{
							Text: "You are a helpful assistant.",
						},
					},
				},
				{
					Role: "user",
					Content: []dashscope.MessageContent{
						{
							Image: "https://dashscope.oss-cn-beijing.aliyuncs.com/images/dog_and_girl.jpeg",
						},
						{
							Text: "这个图片是哪里？",
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	log.With(resp).Debug("resp")
}

func TestDashScope_ChatStream(t *testing.T) {
	client := createClient()
	resp, err := client.ChatStream(context.TODO(), dashscope.ChatRequest{
		Model: "sanle-v1",
		Input: dashscope.ChatInput{
			Prompt: "蓝牙耳机坏了去看牙科还是耳科呢",
		},
	})
	assert.NoError(t, err)

	for res := range resp {
		if res.Code != "" {
			log.With(res).Error("error")
			break
		}

		log.With(res).Debug("res")
	}
}

func TestImageTaskStatus(t *testing.T) {
	client := createClient()

	resp, err := client.ImageTaskStatus(context.TODO(), "512f59d0-d4d4-4720-8fac-b7df8f587670")
	assert.NoError(t, err)

	log.With(resp).Debug("resp")
}
