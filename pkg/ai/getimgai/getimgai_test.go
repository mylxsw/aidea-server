package getimgai_test

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/getimgai"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"gopkg.in/resty.v1"
)

func TestModels(t *testing.T) {
	models, err := createClient().Models(context.TODO(), "", "")
	assert.NoError(t, err)

	// type ArtisticType struct {
	// 	ID             string          `json:"id,omitempty"`
	// 	Name           string          `json:"name,omitempty"`
	// 	Family         string          `json:"family,omitempty"`
	// 	Pipelines      []string        `json:"pipelines,omitempty"`
	// 	BaseResolution ModelResolution `json:"base_resolution,omitempty"`
	// 	Price          float64         `json:"price,omitempty"`
	// 	AuthorURL      string          `json:"author_url,omitempty"`
	// 	LicenseURL     string          `json:"license_url,omitempty"`
	// }
	log.Debugf("total %d models", len(models))
	for _, model := range models {
		fmt.Printf(`{
	ID:             "%s",
	Name:           "%s",
	Family:         "%s",
	Pipelines:      %#v,
	BaseResolution: %#v,
	Price:          %f,
	AuthorURL:      "%s",
	LicenseURL:     "%s",
},`, model.ID, model.Name, model.Family, model.Pipelines, model.BaseResolution, model.Price, model.AuthorURL, model.LicenseURL)
	}
}

func TestAccountBalance(t *testing.T) {
	balance, err := createClient().AccountBalance(context.Background())
	assert.NoError(t, err)

	log.With(balance).Debug("account balance")
}

func TestTextToImage(t *testing.T) {
	resp, err := createClient().TextToImage(context.TODO(), getimgai.TextToImageRequest{
		Prompt: "(masterpiece:1.2),(best quality:1.2), (seaside:1.3), (On the cliff:1.1), (Lighthouse:1.2), (Sailboat:1.1), (Coconut Tree:1.1), illustration,scenery,outdoors, sign, roadsi gn, road, Many stones, tree,sky,cloud, day,horizon,",
		Model:  "icbinp-afterburn",
	})
	assert.NoError(t, err)

	path, err := resp.SaveToLocalFiles("/Users/mylxsw/Downloads")
	assert.NoError(t, err)

	log.Debugf("image saved to %s", path)
}

func TestImageToImage(t *testing.T) {
	imageBase64, err := misc.ImageToRawBase64("/Users/mylxsw/Downloads/2b7d9a35-29c9-3cb4-94ea-9870e91d1928.png")
	assert.NoError(t, err)

	resp, err := createClient().ImageToImage(context.TODO(), getimgai.ImageToImageRequest{
		Image:  imageBase64,
		Prompt: "Miyazaki style",
	})
	assert.NoError(t, err)

	path, err := resp.SaveToLocalFiles("/Users/mylxsw/Downloads")
	assert.NoError(t, err)

	log.Debugf("image saved to %s", path)
}

func TestUpscale(t *testing.T) {
	imageBase64, err := misc.ImageToRawBase64("/Users/mylxsw/Downloads/bedab99a-7f0a-0caa-be7e-b8fbccf4f54c.png")
	assert.NoError(t, err)

	resp, err := createClient().Upscale(context.TODO(), getimgai.UpscaleRequest{
		Image: imageBase64,
		Scale: 4,
	})
	assert.NoError(t, err)

	path, err := resp.SaveToLocalFiles("/Users/mylxsw/Downloads")
	assert.NoError(t, err)

	log.Debugf("image saved to %s", path)
}

func createClient() *getimgai.GetimgAI {
	return getimgai.NewGetimgAIWithResty(
		&config.Config{
			GetimgAIServer: "https://api.getimg.ai",
			GetimgAIKey:    os.Getenv("GETIMG_API_KEY"),
		},
		resty.New(),
	)
}
