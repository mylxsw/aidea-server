package queue_test

import (
	"fmt"
	"github.com/mylxsw/go-utils/assert"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"testing"
)

func TestImageDecode(t *testing.T) {
	f, err := os.Open("/Users/mylxsw/Downloads/ugcd85f6a21-8f99-d038-d002-8f52f03bb03a..jpg")
	assert.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()

	img, _, err := image.DecodeConfig(f)
	assert.NoError(t, err)

	fmt.Println(img.Width, ":", img.Height)
}
