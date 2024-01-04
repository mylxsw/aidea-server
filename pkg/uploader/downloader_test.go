package uploader_test

import (
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/go-utils/must"
	"testing"
)

func TestQueryImageInfo(t *testing.T) {
	info, err := uploader.QueryImageInfo("https://ssl.aicode.cc/ai-server/24/20230811/aigc14995226-1db0-ea85-6f1e-933d19ed01d6.png")
	must.NoError(err)

	t.Log(info)
}

func TestRemoveImageFilter(t *testing.T) {
	fmt.Println(uploader.RemoveImageFilter("https://ssl.aicode.cc/ai-server/24/20230811/aigc14995226-1db0-ea85-6f1e-933d19ed01d6.png"))
	fmt.Println(uploader.RemoveImageFilter("https://ssl.aicode.cc/ai-server/24/20230811/aigc14995226-1db0-ea85-6f1e-933d19ed01d6.png-thumb"))
}
