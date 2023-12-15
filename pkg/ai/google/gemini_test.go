package google_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/google"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"
)

func TestGoogleAI_GeminiChat(t *testing.T) {
	client := google.NewGoogleAI("https://generativelanguage.googleapis.com", os.Getenv("GOOGLE_PALM_API_KEY"))

	encodedImage, mimeType, err := misc.ImageToBase64ImageWithMime("/Users/mylxsw/Downloads/output_dpu7vQA7-_VR_0.png")

	req := google.Request{
		Contents: []google.Message{
			{
				Parts: []google.MessagePart{
					{Text: "这张图上有什么？"},
					{
						InlineData: &google.MessagePartInlineData{MimeType: mimeType, Data: encodedImage},
					},
				},
			},
		},
	}

	resp, err := client.Chat(context.TODO(), google.ModelGeminiProVision, req)
	must.NoError(err)

	t.Log(resp.String())
}

func TestGoogleAI_GeminiChatStream(t *testing.T) {
	client := google.NewGoogleAI("https://generativelanguage.googleapis.com", os.Getenv("GOOGLE_PALM_API_KEY"))

	encodedImage, mimeType, err := misc.ImageToBase64ImageWithMime("/Users/mylxsw/Downloads/output_dpu7vQA7-_VR_0.png")

	req := google.Request{
		Contents: []google.Message{
			{
				Parts: []google.MessagePart{
					{Text: "这张图上有什么？"},
					{
						InlineData: &google.MessagePartInlineData{MimeType: mimeType, Data: encodedImage},
					},
				},
			},
		},
	}

	resp, err := client.ChatStream(context.TODO(), google.ModelGeminiProVision, req)
	must.NoError(err)

	for data := range resp {
		log.With(data).Info("data received")
	}
}
