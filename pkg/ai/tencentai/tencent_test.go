package tencentai_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/tencentai"
	"os"
	"testing"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestTencentAI_ChatStream(t *testing.T) {
	client := tencentai.New(
		os.Getenv("TENCENTCLOUD_SECRET_ID"),
		os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	)

	resp, err := client.ChatStream(context.TODO(), tencentai.NewRequest(
		tencentai.ModelHyllmStd,
		[]tencentai.Message{
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
