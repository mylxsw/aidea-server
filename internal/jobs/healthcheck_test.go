package jobs

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/stabilityai"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/go-utils/assert"
)

func TestQueryStabilityAIBalance(t *testing.T) {
	conf := config.Config{
		StabilityAIServer: []string{"https://api.stability.ai"},
		StabilityAIKey:    os.Getenv("STABILITY_API_KEY"),
	}

	fmt.Println(os.Getenv("STABILITY_API_KEY"))

	st := stabilityai.NewStabilityAIWithClient(&conf, &http.Client{Timeout: 60 * time.Second})

	balance, err := queryStabilityAIBalance(context.TODO(), st)
	assert.NoError(t, err)

	t.Log(balance)
}
