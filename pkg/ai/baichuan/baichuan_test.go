package baichuan_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/baichuan"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"os"
	"testing"
)

func TestBaichuan_Chat(t *testing.T) {
	client := baichuan.NewBaichuanAI(os.Getenv("BAICHUAN_API_KEY"), os.Getenv("BAICHUAN_SECRET_KEY"))

	resp, err := client.Chat(context.TODO(), baichuan.Request{
		Model: baichuan.ModelBaichuan2_53B,
		Messages: []baichuan.Message{
			{
				Role:    "user",
				Content: "你是一名占卜师，我给你名字，你帮我占卜运势",
			},
			{
				Role:    "assistant",
				Content: "OK，请告诉我你的名字",
			},
			{
				Role:    "user",
				Content: "我的名字是李逍遥，请帮我占卜一下运势",
			},
		},
		Parameters: baichuan.Parameters{
			WithSearchEnhance: true,
		},
	})
	assert.NoError(t, err)

	if resp.Code != 0 {
		log.With(resp).Error("resp error")
		return
	}

	log.With(resp).Debug("resp")
}

func TestBaichuan_ChatStream(t *testing.T) {
	client := baichuan.NewBaichuanAI(os.Getenv("BAICHUAN_API_KEY"), os.Getenv("BAICHUAN_SECRET_KEY"))

	resp, err := client.ChatStream(context.TODO(), baichuan.Request{
		Model: baichuan.ModelBaichuan2_53B,
		Messages: []baichuan.Message{
			{
				Role:    "user",
				Content: "你是一名占卜师，我给你名字，你帮我占卜运势",
			},
			{
				Role:    "assistant",
				Content: "OK，请告诉我你的名字",
			},
			{
				Role:    "user",
				Content: "我的名字是李逍遥，请帮我占卜一下运势",
			},
		},
		Parameters: baichuan.Parameters{
			WithSearchEnhance: true,
		},
	})
	assert.NoError(t, err)

	for res := range resp {
		if res.Code != 0 {
			log.With(res).Error("resp error")
			return
		}

		for _, msg := range res.Data.Messages {
			log.With(msg).Debugf("-> %s", msg.Content)
		}
	}
}
