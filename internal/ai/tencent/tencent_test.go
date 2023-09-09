package tencent_test

import (
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/internal/ai/tencent"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestTencentArt_ImageToImage(t *testing.T) {
	client, err := tencent.NewTencentArt(os.Getenv("TENCENTCLOUD_SECRET_ID"), os.Getenv("TENCENTCLOUD_SECRET_KEY"))
	assert.NoError(t, err)

	base64Image, err := helper.ImageToBase64Image("/Users/mylxsw/Downloads/Xnip2023-08-21_15-50-50.png")
	assert.NoError(t, err)

	req := tencent.ImageToImageRequest{
		ImageBase64: base64Image,
		Style:       "110",
	}

	image, err := client.ImageToImage(req)
	assert.NoError(t, err)

	log.WithFields(log.Fields{"image": image}).Info("image to image")
}
