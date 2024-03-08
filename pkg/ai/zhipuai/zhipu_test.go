package zhipuai_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/zhipuai"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"
)

func TestZhipuAI_Chat(t *testing.T) {
	client := zhipuai.NewZhipuAI(os.Getenv("ZHIPUAI_API_KEY"))
	resp, err := client.Chat(context.TODO(), zhipuai.ChatRequest{
		Model: zhipuai.ModelGLM4,
		Messages: []any{
			zhipuai.Message{
				Role:    "user",
				Content: "你好",
			},
		},
	})
	must.NoError(err)

	log.With(resp).Debug("response")
}

func TestZhipuAI_ChatStream(t *testing.T) {
	client := zhipuai.NewZhipuAI(os.Getenv("ZHIPUAI_API_KEY"))
	resp, err := client.ChatStream(context.TODO(), zhipuai.ChatRequest{
		Model: zhipuai.ModelGLM4,
		Messages: []any{
			zhipuai.Message{
				Role:    "user",
				Content: "你好",
			},
		},
	})
	must.NoError(err)

	for data := range resp {
		log.With(data).Debug("response")
	}
}

func TestZhipuAI_ChatStreamV(t *testing.T) {
	client := zhipuai.NewZhipuAI(os.Getenv("ZHIPUAI_API_KEY"))
	resp, err := client.ChatStream(context.TODO(), zhipuai.ChatRequest{
		Model: zhipuai.ModelGLM4V,
		Messages: []any{
			zhipuai.MultipartMessage{
				Role: "user",
				Content: []zhipuai.MultipartContent{
					{
						Type: "text",
						Text: "这张图中描绘了啥子",
					},
					{
						Type: "image_url",
						ImageURL: &zhipuai.MultipartContentImage{
							URL: "https://ssl.aicode.cc/ai-server/assets/styles/style-scene.jpg-avatar",
						},
					},
				},
			},
		},
	})
	must.NoError(err)

	for data := range resp {
		log.With(data).Debug("response")
	}
}
