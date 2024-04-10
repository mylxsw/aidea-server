package voice

import (
	"context"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"os"
	"testing"
)

func TestMiniMaxVoiceClient_TextToSpeech(t *testing.T) {
	client := NewMiniMaxVoiceClient(os.Getenv("MINIMAX_API_KEY"), os.Getenv("MINIMAX_GROUP_ID"))
	resp, err := client.Text2Voice(context.TODO(), "你好，世界。Hello, World.", TypeFemale1)
	assert.NoError(t, err)

	log.With(resp).Debug("response")

}
