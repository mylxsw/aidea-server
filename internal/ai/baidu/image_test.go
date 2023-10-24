package baidu_test

import (
	"context"
	"github.com/mylxsw/aidea-server/internal/ai/baidu"
	"github.com/mylxsw/go-utils/assert"
	"os"
	"testing"
)

func TestImageStyleTrans(t *testing.T) {
	client := baidu.NewBaiduImageAI(os.Getenv("BAIDU_IMAGE_API_KEY"), os.Getenv("BAIDU_IMAGE_SECRET"))

	resp, err := client.ImageStyleTrans(context.TODO(),
		baidu.ImageStyleTransRequest{
			URL:    "https://ssl.aicode.cc/ai-server/24/20230920/ugc62e48808-cd33-65a0-ef27-70c67b3a4d0a..jpeg",
			Option: "scream",
		},
	)
	assert.NoError(t, err)

	f, err := os.Create("/tmp/test.jpg")
	assert.NoError(t, err)

	defer f.Close()

	_, err = resp.WriteTo(f)
	assert.NoError(t, err)
}

func TestSelfieAnime(t *testing.T) {
	client := baidu.NewBaiduImageAI(os.Getenv("BAIDU_IMAGE_API_KEY"), os.Getenv("BAIDU_IMAGE_SECRET"))

	resp, err := client.SelfieAnime(context.TODO(),
		baidu.SelfieAnimeRequest{
			URL:    "https://ssl.aicode.cc/ai-server/24/20230920/ugc62e48808-cd33-65a0-ef27-70c67b3a4d0a..jpeg",
			Type:   "anime_mask",
			MaskID: 4,
		},
	)
	assert.NoError(t, err)

	f, err := os.Create("/tmp/test.jpg")
	assert.NoError(t, err)

	defer f.Close()

	_, err = resp.WriteTo(f)
	assert.NoError(t, err)
}

func TestColourize(t *testing.T) {
	client := baidu.NewBaiduImageAI(os.Getenv("BAIDU_IMAGE_API_KEY"), os.Getenv("BAIDU_IMAGE_SECRET"))

	resp, err := client.Colourize(context.TODO(),
		baidu.SimpleImageRequest{
			URL: "https://ssl.aicode.cc/ai-server/24/20231013/ugc1351a78d-e605-c895-289b-6833ed57c9dd..jpeg",
		},
	)
	assert.NoError(t, err)

	f, err := os.Create("/tmp/test.jpg")
	assert.NoError(t, err)

	defer f.Close()

	_, err = resp.WriteTo(f)
	assert.NoError(t, err)
}

func TestImageQualityEnhance(t *testing.T) {
	client := baidu.NewBaiduImageAI(os.Getenv("BAIDU_IMAGE_API_KEY"), os.Getenv("BAIDU_IMAGE_SECRET"))

	resp, err := client.QualityEnhance(context.TODO(),
		baidu.SimpleImageRequest{
			URL: "https://ssl.aicode.cc/ai-server/24/20231013/ugc1351a78d-e605-c895-289b-6833ed57c9dd..jpeg",
		},
	)
	assert.NoError(t, err)

	f, err := os.Create("/tmp/test.jpg")
	assert.NoError(t, err)

	defer f.Close()

	_, err = resp.WriteTo(f)
	assert.NoError(t, err)
}
