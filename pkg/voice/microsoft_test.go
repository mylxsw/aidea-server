package voice_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/voice"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"
)

func TestMicrosoftSpeech_TextToSpeech(t *testing.T) {
	speak := voice.NewSpeak(
		"zh-CN-shandong-YunxiangNeural",
		"Female",
		`这张图片展示的是一个猫的脸部，但看起来被艺术化地修改过，给它加上了一种科幻或者朋克风格的外观。猫的眼睛被两个橙色的圆形镜片覆盖，这些镜片可能代表着某种虚构的高科技眼镜或机械眼睛。猫的头部还装饰了一些看似金属的饰品和装置，这增加了一种工业或蒸汽朋克的感觉。整体上，这个图像是一种将动物与科幻元素结合的创意设计。`,
	)

	sp := voice.NewMicrosoftSpeech(os.Getenv("MICROSOFT_SPEECH_KEY"), "eastus")
	data, err := sp.TextToSpeech(context.TODO(), speak)
	must.NoError(err)

	must.NoError(os.WriteFile("/tmp/voice.mp3", data, 0644))
}
