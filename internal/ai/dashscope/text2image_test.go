package dashscope_test

import (
	"context"
	"testing"
	"time"

	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestText2Image(t *testing.T) {
	client := createClient()

	resp, err := client.Text2Image(context.TODO(), dashscope.Text2ImageRequest{
		Model: dashscope.ImageModelText2Image,
		Input: dashscope.Text2ImageInput{
			Prompt: "画一张鲁迅暴打周树人的画",
		},
		Parameters: dashscope.Text2ImageParameters{
			Style: dashscope.Text2ImageStyleAnime,
			N:     1,
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
