package dashscope_test

import (
	"context"
	"testing"
	"time"

	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestImageGeneration(t *testing.T) {
	client := createClient()

	resp, err := client.ImageGeneration(context.TODO(), dashscope.ImageGenerationRequest{
		Model: dashscope.ImageModelPersonRepaint,
		Input: dashscope.ImageGenerationRequestInput{
			ImageURL:   "https://ssl.aicode.cc/ai-server/24/20231011/ugc17960131-839e-768c-6305-38e7a829fc9b..jpeg",
			StyleIndex: dashscope.ImageStyleFuture,
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
