package dashscope_test

import (
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestDashScope_Chat(t *testing.T) {
	client := dashscope.New(os.Getenv("ALI_LINGJI_API_KEY"))
	resp, err := client.Chat(dashscope.ChatRequest{
		Model: "qwen-v1",
		Input: dashscope.ChatInput{
			Prompt: "你好",
		},
	})
	assert.NoError(t, err)

	log.With(resp).Debug("resp")
}

func TestDashScope_ChatStream(t *testing.T) {
	client := dashscope.New(os.Getenv("ALI_LINGJI_API_KEY"))
	resp, err := client.ChatStream(dashscope.ChatRequest{
		Model: "qwen-v1",
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
