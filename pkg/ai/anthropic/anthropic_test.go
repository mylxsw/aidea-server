package anthropic_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/anthropic"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestAnthropic_Chat(t *testing.T) {
	client := anthropic.New("", os.Getenv("ANTHROPIC_API_KEY"), http.DefaultClient)

	resp, err := client.Chat(context.TODO(), anthropic.MessageRequest{
		Model: anthropic.ModelClaude3Haiku,
		Messages: []anthropic.Message{
			anthropic.NewTextMessage("user", "你是一名占卜师，我给你名字，你帮我占卜运势"),
			anthropic.NewTextMessage("assistant", "OK，请告诉我你的名字"),
			anthropic.NewTextMessage("user", "我的名字是李逍遥，请帮我占卜一下运势"),
		},
	})
	assert.NoError(t, err)

	if resp.Error != nil && resp.Error.Type != "" {
		log.With(resp.Error).Error("resp error")
		return
	}

	log.With(resp).Debug("resp")
}

func TestAnthropic_ChatStream(t *testing.T) {
	client := anthropic.New("", os.Getenv("ANTHROPIC_API_KEY"), http.DefaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.ChatStream(ctx, anthropic.MessageRequest{
		Model: "claude-3-7-sonnet-20250219",
		Messages: []anthropic.Message{
			anthropic.NewTextMessage("user", "你是一名占卜师，我给你名字，你帮我占卜运势"),
			anthropic.NewTextMessage("assistant", "OK，请告诉我你的名字"),
			anthropic.NewTextMessage("user", "我的名字是李逍遥，请帮我占卜一下运势"),
		},
		Thinking: &anthropic.Thinking{
			Type:         "enabled",
			BudgetTokens: 16000,
		},
		MaxTokens: 20000,
	})
	assert.NoError(t, err)

	var result string
	var thinking string
	for res := range resp {
		if res.Error != nil && res.Error.Type != "" {
			log.With(res.Error).Error("resp error")
			return
		}

		switch res.Delta.Type {
		case "text_delta":
			result += res.Delta.Text
		case "thinking_delta":
			thinking += res.Delta.Thinking
		}
	}

	fmt.Println("---------- Thinking ----------")
	fmt.Println(thinking)
	fmt.Println("---------- Result ----------")
	fmt.Println(result)

}
