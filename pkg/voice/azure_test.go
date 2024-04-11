package voice_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/voice"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"
)

func TestMicrosoftSpeech_TextToSpeech(t *testing.T) {
	sp := voice.NewAzureVoiceEngine(os.Getenv("MICROSOFT_SPEECH_KEY"), "eastus", "/tmp")
	localPath, err := sp.Text2Voice(context.TODO(), "你好，世界。Hello, World.", voice.TypeFemale1)
	must.NoError(err)

	fmt.Println(localPath)
}
