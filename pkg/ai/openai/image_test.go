package openai_test

import (
	"context"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"os"
	"testing"
)

func TestImageClient_CreateImage(t *testing.T) {
	openaiURL := os.Getenv("OPENAI_URL")
	openaiKey := os.Getenv("OPENAI_TOKEN")

	client := openai2.NewDalleImageClient(&openai2.Config{
		Enable:        true,
		OpenAIServers: []string{openaiURL},
		OpenAIKeys:    []string{openaiKey},
	}, nil)

	resp, err := client.CreateImage(context.TODO(), openai2.ImageRequest{
		Prompt:         "一直在努力的人，终会有回报",
		Model:          "dall-e-3",
		N:              1,
		Size:           "1024x1024",
		ResponseFormat: "b64_json",
	})
	assert.NoError(t, err)

	log.With(resp.Error).Debug("painting response")
	for _, img := range resp.Data {
		img.Base64JSON = misc.SubString(img.Base64JSON, 100)
		log.With(img).Debug("painting image")
	}
}
