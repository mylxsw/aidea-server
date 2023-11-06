package oneapi_test

import (
	"context"
	"github.com/mylxsw/aidea-server/internal/ai/oneapi"
	oai "github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"github.com/sashabaranov/go-openai"
	"os"
	"testing"
	"time"
)

func TestOneAPI_ChatStream(t *testing.T) {
	conf := &oai.Config{
		Enable:        true,
		OpenAIServers: []string{os.Getenv("ONEAPI_URL")},
		OpenAIKeys:    []string{os.Getenv("ONEAPI_KEY")},
	}
	client := oneapi.New(oai.NewOpenAIClient(conf, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, err := client.ChatStream(ctx, openai.ChatCompletionRequest{
		Model: "chatglm_turbo",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃",
			},
			{
				Role:    "user",
				Content: "老铁，最近怎么样？",
			},
		},
	})

	assert.NoError(t, err)

	for res := range response {
		log.With(res).Debugf("-> %s", res.ChatResponse.Choices[0].Delta.Content)
	}
}
