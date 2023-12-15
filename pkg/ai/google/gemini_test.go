package google_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/google"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"
)

func TestGoogleAI_GeminiChat(t *testing.T) {
	client := google.NewGoogleAI("https://generativelanguage.googleapis.com", os.Getenv("GOOGLE_PALM_API_KEY"))

	req := google.Request{
		Contents: []google.Message{
			{
				Parts: []google.MessagePart{
					{Text: "Hello, world"},
				},
			},
		},
	}

	resp, err := client.GeminiChat(context.TODO(), req)
	must.NoError(err)

	t.Log(resp.String())
}

func TestGoogleAI_GeminiChatStream(t *testing.T) {
	client := google.NewGoogleAI("https://generativelanguage.googleapis.com", os.Getenv("GOOGLE_PALM_API_KEY"))

	req := google.Request{
		Contents: []google.Message{
			{
				Parts: []google.MessagePart{
					{Text: "帮我写一篇主题为天上人间的作文"},
				},
			},
		},
	}

	resp, err := client.GeminiChatStream(context.TODO(), req)
	must.NoError(err)

	for data := range resp {
		log.With(data).Info("data received")
	}
}
