package leap_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/leap"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/go-utils/assert"
	"gopkg.in/resty.v1"
)

func TestLeap(t *testing.T) {
	conf := config.Config{
		LeapAIServers: []string{"https://api.tryleap.ai"},
		LeapAIKey:     os.Getenv("LEAPAI_API_KEY"),
	}
	ai := leap.NewLeapAIWithClient(
		&conf,
		&http.Client{Timeout: 60 * time.Second},
		resty.New().SetTimeout(60*time.Second),
	)

	resp, err := ai.RemixImageUpload(context.TODO(), leap.ModelRealisticVision_v4_0, &leap.RemixImageRequest{
		Files:  "/Users/mylxsw/Downloads/cover.jpg",
		Prompt: "blue hair color",
	})
	assert.NoError(t, err)

	t.Log(resp)
}
