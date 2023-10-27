package anthropic_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mylxsw/aidea-server/internal/ai/anthropic"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestAnthropic_Chat(t *testing.T) {
	client := anthropic.New("", os.Getenv("ANTHROPIC_API_KEY"), http.DefaultClient)

	resp, err := client.Chat(context.TODO(), anthropic.NewRequest(anthropic.ModelClaudeInstant, []anthropic.Message{
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

	if resp.Error != nil && resp.Error.Type != "" {
		log.With(resp.Error).Error("resp error")
		return
	}

	log.With(resp).Debug("resp")
}

func TestAnthropic_ChatStream(t *testing.T) {
	client := anthropic.New("", os.Getenv("ANTHROPIC_API_KEY"), http.DefaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.ChatStream(ctx, anthropic.NewRequest(anthropic.ModelClaudeInstant, []anthropic.Message{
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
		if res.Error != nil && res.Error.Type != "" {
			log.With(res.Error).Error("resp error")
			return
		}

		fmt.Print(res.Completion)
	}
}
