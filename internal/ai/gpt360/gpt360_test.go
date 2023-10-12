package gpt360_test

import (
	"context"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/internal/ai/gpt360"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestGPT360_Chat(t *testing.T) {
	client := gpt360.NewGPT360(os.Getenv("GPT360_API_KEY"))
	resp, err := client.Chat(context.TODO(), gpt360.ChatRequest{
		Model: gpt360.Model360GPT_S2_V9,
		Messages: []gpt360.Message{
			{
				Role:    "system",
				Content: "你是一名占卜师，我给你名字，你帮我占卜运势",
			},
			{
				Role:    "user",
				Content: "我的名字是李逍遥",
			},
		},
	})
	assert.NoError(t, err)

	log.With(resp).Debug("resp")
}

func TestGPT360_ChatStream(t *testing.T) {
	client := gpt360.NewGPT360(os.Getenv("GPT360_API_KEY"))
	resp, err := client.ChatStream(context.TODO(), gpt360.ChatRequest{
		Model: gpt360.Model360GPT_S2_V9,
		Messages: []gpt360.Message{
			{
				Role:    "system",
				Content: "你是一名占卜师，我给你名字，你帮我占卜运势",
			},
			{
				Role:    "user",
				Content: "我的名字是李逍遥",
			},
		},
	})
	assert.NoError(t, err)

	for res := range resp {
		if res.Error.Code != "" {
			log.With(res).Error("error")
			break
		}

		log.With(res).Debug("res")
	}
}
