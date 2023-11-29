package dashscope_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"testing"
)

func TestDashScope_FaceChainDetect(t *testing.T) {
	client := createClient()
	resp, err := client.FaceChainDetect(
		context.TODO(),
		dashscope.FaceChainPersonDetectInput{
			Images: []string{},
		},
	)
	assert.NoError(t, err)

	log.With(resp).Debug("facechain detect result")
}
