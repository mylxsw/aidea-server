package dashscope_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"testing"
	"time"
)

func TestWordArtTexture(t *testing.T) {
	client := createClient()

	resp, err := client.WordArtTexture(context.TODO(), dashscope.WordArtTextureRequest{
		Model: dashscope.WordArtTextureModel,
		Input: dashscope.WordArtTextureRequestInput{
			Text: &dashscope.WordArtTextureRequestInputText{
				FontName:    "dongfangdakai",
				TextContent: "管宜尧",
			},
			TextureStyle: "material",
			Prompt:       "乐高积木",
		},
		Parameters: dashscope.WordArtTextureRequestParameters{
			ImageShortSize: 512,
			N:              1,
		},
	})
	assert.NoError(t, err)

	log.With(resp).Debug("resp")

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for {
		select {
		case <-ticker.C:
			taskResp, err := client.ImageTaskStatus(context.TODO(), resp.Output.TaskID)
			assert.NoError(t, err)

			log.With(taskResp).Debug("resp")

			if taskResp.Output.TaskStatus == dashscope.TaskStatusSucceeded || taskResp.Output.TaskStatus == dashscope.TaskStatusFailed {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func TestWordArtTextureTask(t *testing.T) {
	taskID := "66d6c959-9e32-42cd-845e-60c70236fe4b"

	client := createClient()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for {
		select {
		case <-ticker.C:
			taskResp, err := client.ImageTaskStatus(context.TODO(), taskID)
			assert.NoError(t, err)

			log.With(taskResp).Debug("resp")

			if taskResp.Output.TaskStatus == dashscope.TaskStatusSucceeded || taskResp.Output.TaskStatus == dashscope.TaskStatusFailed {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
