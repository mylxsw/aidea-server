package image_test

import (
	"github.com/mylxsw/aidea-server/pkg/image"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"
)

func TestImager_TextImage(t *testing.T) {
	imager := image.New("/Users/mylxsw/Workspace/codes/resources/fonts/JingNanMaiYuanTi-2.otf")
	data, err := imager.TextImage("管\n宜\n尧\n早", 768)
	if err != nil {
		panic(err)
	}

	must.NoError(os.WriteFile("data.png", data, 0644))
}

func TestImager_QR(t *testing.T) {
	imager := image.New("/Users/mylxsw/Workspace/codes/resources/fonts/JingNanMaiYuanTi-2.otf")
	data, err := imager.QR("https://aidea.aicode.cc", 768)
	if err != nil {
		panic(err)
	}

	must.NoError(os.WriteFile("data.png", data, 0644))
}
