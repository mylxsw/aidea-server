package voice_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/voice"
	"github.com/mylxsw/go-utils/assert"
)

func TestVocice(t *testing.T) {

	client := voice.NewVoice(&config.Config{
		StorageAppKey:    os.Getenv("QINIU_ACCESS_KEY"),
		StorageAppSecret: os.Getenv("QINIU_SECRET_KEY"),
	}, nil)

	res, err := client.Text2Voice(context.TODO(), voice.VoiceRequest{
		Spkid:   13,
		Content: "语音合成可将文本转化成拟人化语音的一类功能，采用先进的深度神经网络模型技术，合成效果自然流畅，合成度快，部署成本低，并提供多语种、多音色可供选择，满足不同业务场景需求，可广泛应用于新闻播报、小说、客服、智能硬件等场景。",
	})
	assert.NoError(t, err)
	fmt.Println(res)
}

func TestPayCount(t *testing.T) {
	fmt.Println(int64(math.Ceil(float64(1) / 2.0)))
	fmt.Println(int64(math.Ceil(float64(2) / 2.0)))
	fmt.Println(int64(math.Ceil(float64(3) / 2.0)))
}
