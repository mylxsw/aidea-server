package stabilityai_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/stabilityai"
)

func TestAccountCredits(t *testing.T) {
	conf := config.Config{
		StabilityAIServer: []string{"https://api.stability.ai"},
		StabilityAIKey:    os.Getenv("STABILITY_API_KEY"),
	}

	st := stabilityai.NewStabilityAIWithClient(&conf, &http.Client{Timeout: 60 * time.Second})
	res, err := st.AccountBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	t.Log(res)
}

func TestScale(t *testing.T) {
	conf := config.Config{
		StabilityAIServer: []string{"https://api.stability.ai"},
		StabilityAIKey:    os.Getenv("STABILITY_API_KEY"),
	}

	st := stabilityai.NewStabilityAIWithClient(&conf, &http.Client{Timeout: 60 * time.Second})
	resp, err := st.Upscale(context.TODO(), stabilityai.UpscaleEsganV1X2PlusModel, "/Users/mylxsw/ResilioSync/AI/IMAGES/1.jpg", 2000, 0)
	if err != nil {
		t.Fatal(err)
	}

	res, err := resp.SaveToLocalFiles(context.TODO(), "/Users/mylxsw/Downloads")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(res)
}

func TestImageToImage(t *testing.T) {
	conf := config.Config{
		StabilityAIServer: []string{"https://api.stability.ai"},
		StabilityAIKey:    os.Getenv("STABILITY_API_KEY"),
	}

	st := stabilityai.NewStabilityAIWithClient(&conf, &http.Client{Timeout: 60 * time.Second})
	resp, err := st.ImageToImage(context.TODO(), "stable-diffusion-v1-5", stabilityai.ImageToImageRequest{
		InitImage:   "/Users/mylxsw/ResilioSync/AI/APP Icons/app.png",
		TextPrompt:  "add some light to image",
		StylePreset: "neon-punk",
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := resp.SaveToLocalFiles(context.TODO(), "/Users/mylxsw/Downloads")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(res)
}

func TestTextToImage(t *testing.T) {
	conf := config.Config{
		StabilityAIServer: []string{"https://api.stability.ai"},
		StabilityAIKey:    os.Getenv("STABILITY_API_KEY"),
	}

	st := stabilityai.NewStabilityAIWithClient(&conf, &http.Client{Timeout: 60 * time.Second})
	resp, err := st.TextToImage("stable-diffusion-v1-5", stabilityai.TextToImageRequest{
		TextPrompts: []stabilityai.TextPrompts{
			{Text: "a small cat"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := resp.SaveToLocalFiles(context.TODO(), "/Users/mylxsw/Downloads")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(res)
}
