package dashscope_test

import (
	"context"
	dashscope2 "github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"testing"
	"time"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestStableDiffusion(t *testing.T) {
	client := createClient()

	resp, err := client.StableDiffusion(context.TODO(), dashscope2.StableDiffusionRequest{
		Model: dashscope2.ImageModelSDXL,
		Input: dashscope2.StableDiffusionInput{
			Prompt: "no humans, outdoors, scenery, power lines, clouds, sky, poles, 1 houses, buildings, grass, plants, blue sky, cloudy sky, windows, trees, day, distant view, nice clouds, depth of field, distant view, (Illustration: 1.0), masterpiece, best quality",
		},
		Parameters: dashscope2.StableDiffusionParameters{
			N: 1,
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

			if taskResp.Output.TaskStatus == dashscope2.TaskStatusSucceeded || taskResp.Output.TaskStatus == dashscope2.TaskStatusFailed {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
