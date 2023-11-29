package fromston_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/fromston"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestModels(t *testing.T) {
	art := createArtClient()

	models, err := art.Models(context.TODO())
	assert.NoError(t, err)

	for _, model := range models {
		fmt.Printf("%-10d | %10s | %-70s | %s\n", model.ModelID, model.Type, model.Name, model.ArtistStyle)
	}
}

func TestCreateImage(t *testing.T) {
	art := createArtClient()

	req := fromston.GenImageRequest{
		Prompt:    "层层叠叠的玫瑰花开在山坡上",
		Width:     512,
		Height:    512,
		ModelType: "third",
		ModelID:   30,
		Multiply:  1,
		Addition: &fromston.GenImageAddition{
			CfgScale: 7,
			ImgFmt:   "png",
		},
	}

	resp, err := art.GenImage(context.TODO(), req)
	assert.NoError(t, err)

	log.With(resp).Debug("generate image response")
}

func TestQueryTask(t *testing.T) {
	art := createArtClient()

	tasks, err := art.QueryTasks(context.TODO(), []string{"04e46064f3762488"})
	assert.NoError(t, err)

	log.With(tasks).Debug("query task response")
}

func createArtClient() *fromston.Fromston {
	art := fromston.NewFromston(&config.Config{
		FromstonServer: "https://ston.6pen.art",
		FromstonKey:    os.Getenv("FROM_STON_API_KEY"),
	})
	return art
}

func TestUploadFile(t *testing.T) {
	art := createArtClient()

	res, err := art.UploadImage(context.TODO(), "/Users/mylxsw/Downloads/20230710/WechatIMG33082.jpg")
	assert.NoError(t, err)

	fmt.Println(res)
}
