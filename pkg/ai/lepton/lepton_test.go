package lepton_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/ai/lepton"
	"github.com/mylxsw/aidea-server/pkg/image"
	"github.com/mylxsw/go-utils/assert"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"
)

func TestLepton_QRImageGenerate(t *testing.T) {
	conf := config.Config{
		LeptonAIQRServers: []string{"https://aiqr.lepton.run"},
		LeptonAIKeys:      []string{os.Getenv("LEPTON_API_KEY")},
	}

	fontPath := "/Users/mylxsw/Workspace/codes/resources/fonts/JingNanMaiYuanTi-2.otf"
	maskData := must.Must(image.New(fontPath).QR("https://aidea.aicode.cc", 768))

	client := lepton.Default(&conf)
	resp, err := client.ImageGenerate(context.TODO(), lepton.QRImageRequest{
		ControlImage:      base64.StdEncoding.EncodeToString(maskData),
		Model:             "prime",
		Prompt:            "An ancient and mysterious forest, illuminated by moonlight, with towering trees, mossy stones, and a clear stream, inspired by fantasy novels, high resolution and rich in detail",
		ControlImageRatio: 0.8,
		ControlWeight:     1.35,
		GuidanceStart:     0.3,
		GuidanceEnd:       0.95,
		Seed:              -1,
		Steps:             30,
		CfgScale:          7,
		NumImages:         1,
	})
	assert.NoError(t, err)
	if err == nil {
		res, err := resp.SaveToLocalFiles(context.TODO(), "/tmp/")
		assert.NoError(t, err)

		fmt.Println(res)
	}
}
