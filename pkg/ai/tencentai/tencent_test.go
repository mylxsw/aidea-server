package tencentai_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/tencentai"
	"os"
	"strconv"
	"testing"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"github.com/mylxsw/go-utils/must"
)

func TestTencentAI_Chat(t *testing.T) {
	client := tencentai.New(
		must.Must(strconv.Atoi(os.Getenv("TENCENTCLOUD_APPID"))),
		os.Getenv("TENCENTCLOUD_SECRET_ID"),
		os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	)

	resp, err := client.Chat(context.TODO(), tencentai.NewRequest([]tencentai.Message{
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
			Content: "我的名字是李逍遥",
		},
	}))
	assert.NoError(t, err)

	if resp.Error.Code != 0 {
		log.With(resp.Error).Error("resp error")
		return
	}

	log.With(resp).Debug("resp")
}

func TestTencentAI_ChatStream(t *testing.T) {
	client := tencentai.New(
		must.Must(strconv.Atoi(os.Getenv("TENCENTCLOUD_APPID"))),
		os.Getenv("TENCENTCLOUD_SECRET_ID"),
		os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	)

	resp, err := client.ChatStream(context.TODO(), tencentai.NewRequest([]tencentai.Message{
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
	}))
	assert.NoError(t, err)

	for res := range resp {
		if res.Error.Code != 0 {
			log.With(res).Error("error")
			break
		}

		fmt.Print(res.Choices[0].Delta.Content)
	}
}

func TestTencentMessageFix(t *testing.T) {
	messages := tencentai.Messages{
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
			Content: "我的名字是李逍遥",
		},
	}

	log.With(messages.Fix()).Debug("messages")
}
