package sensenova_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/sensenova"
	"os"
	"testing"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestSenseNova_Chat(t *testing.T) {
	client := sensenova.New(os.Getenv("SENSENOVA_KEY_ID"), os.Getenv("SENSENOVA_KEY_SECRET"))

	resp, err := client.Chat(context.TODO(), sensenova.Request{
		Model: sensenova.ModelNovaPtcXLV1,
		Messages: []sensenova.Message{
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

func TestSenseNova_ChatStream(t *testing.T) {
	client := sensenova.New(os.Getenv("SENSENOVA_KEY_ID"), os.Getenv("SENSENOVA_KEY_SECRET"))
	resp, err := client.ChatStream(context.TODO(), sensenova.Request{
		Model: sensenova.ModelNovaPtcXLV1,
		Messages: []sensenova.Message{
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
		if res.Error.Code != 0 {
			log.With(res).Error("error")
			break
		}

		log.With(res).Debug("res")
	}
}
