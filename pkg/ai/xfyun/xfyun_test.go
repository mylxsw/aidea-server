package xfyun_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/xfyun"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"

	"github.com/mylxsw/go-utils/assert"
)

func TestXFYunAI_ChatStream(t *testing.T) {
	client := xfyun.New(os.Getenv("XFYUN_APPID"), os.Getenv("XFYUN_API_KEY"), os.Getenv("XFYUN_API_SECRET"))

	messages := []xfyun.Message{
		{Role: xfyun.RoleUser, Content: "蓝牙耳机坏了去看牙科还是耳科呢？"},
	}

	resp, err := client.ChatStream(context.TODO(), xfyun.ModelGeneralV3, messages)
	assert.NoError(t, err)

	for r := range resp {
		fmt.Print(r.Payload.Choices.Text[0].Content)
	}

	fmt.Println()
}

func TestXFYunAI_ImageChatStream(t *testing.T) {
	client := xfyun.New(os.Getenv("XFYUN_APPID"), os.Getenv("XFYUN_API_KEY"), os.Getenv("XFYUN_API_SECRET"))

	res, err := client.DescribeImage(context.TODO(), "https://stable-diffusion-art.com/wp-content/uploads/2023/05/image-161.png", false)
	must.NoError(err)

	fmt.Println(res)
}
