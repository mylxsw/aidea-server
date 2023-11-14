package deepai_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/deepai"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/go-utils/assert"
)

func TestDeepAI_Upscale(t *testing.T) {
	client := deepai.NewDeepAIRaw(&config.Config{
		DeepAIServer:    []string{"https://deepai.88888888.cool"},
		DeepAIKey:       os.Getenv("DEEPAI_API_KEY"),
		DeepAIAutoProxy: false,
	})

	response, err := client.Upscale(context.TODO(), "https://ssl.aicode.cc/ai-server/24/20230814/ugc9fbd42ce-724a-fc40-e725-b597a57c5511..jpg")
	assert.NoError(t, err)

	fmt.Println(response.ID)
	fmt.Println(response.OutputURL)
}
