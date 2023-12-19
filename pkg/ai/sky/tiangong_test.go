package sky_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/sky"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"
)

func TestTiangong_Chat(t *testing.T) {
	client := sky.New(os.Getenv("TIANGONG_APP_KEY"), os.Getenv("TIANGONG_APP_SECRET"))

	req := sky.Request{
		Messages: []sky.Message{
			{
				Role:    "user",
				Content: "你是一名占卜师，我给你名字，你帮我占卜运势",
			},
			{
				Role:    "bot",
				Content: "OK，请告诉我你的名字",
			},
			{
				Role:    "user",
				Content: "我的名字是李逍遥，请帮我占卜一下运势",
			},
		},
		Model: sky.ModelSkyChatMegaVerse,
	}

	resp, err := client.Chat(context.TODO(), req)
	must.NoError(err)

	t.Log(resp)
}

func TestTiangong_ChatStream(t *testing.T) {
	client := sky.New(os.Getenv("TIANGONG_APP_KEY"), os.Getenv("TIANGONG_APP_SECRET"))

	req := sky.Request{
		Messages: []sky.Message{
			{
				Role:    "user",
				Content: "帮我写一篇关于党的十三大的文章",
			},
		},
		Model: sky.ModelSkyChatMegaVerse,
	}

	resp, err := client.ChatStream(context.TODO(), req)
	assert.NoError(t, err)

	for res := range resp {
		if res.Code != 0 {
			log.With(res).Error("resp error")
			return
		}

		if res.RespData.IsSensitive() {
			log.With(res).Error("resp is sensitive")
			return
		}

		log.With(res).Debugf("-> %s", res.RespData.Reply)
	}
}
