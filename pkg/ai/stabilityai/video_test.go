package stabilityai_test

import (
	"context"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/ai/stabilityai"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestStabilityAI_ImageToVideo(t *testing.T) {
	conf := config.Config{
		StabilityAIServer: []string{"https://api.stability.ai"},
		StabilityAIKey:    os.Getenv("STABILITY_API_KEY"),
	}

	req := stabilityai.VideoRequest{
		ImagePath: "/Users/mylxsw/Downloads/IMG_8649.png",
	}

	st := stabilityai.NewStabilityAIWithClient(&conf, &http.Client{Timeout: 60 * time.Second})
	resp, err := st.ImageToVideo(context.TODO(), req)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)

	time.Sleep(5 * time.Second)

	for {
		res, err := st.ImageToVideoResult(context.TODO(), resp.ID)
		if err != nil {
			t.Fatal(err)
		}

		if res.Video != "" {
			filepath, err := res.SaveToLocalFiles(context.TODO(), "/tmp")
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("saved as %s", filepath)
			break
		}

		if res.Status == "in-progress" {
			t.Log("in progress")
			time.Sleep(5 * time.Second)
			continue
		}

		t.Log("unknown status")
		time.Sleep(5 * time.Second)
	}
}

func TestStabilityAI_ImageToVideoResult(t *testing.T) {
	conf := config.Config{
		StabilityAIServer: []string{"https://api.stability.ai"},
		StabilityAIKey:    os.Getenv("STABILITY_API_KEY"),
	}

	id := "57e47215ec64c9ff7c5f3e850e9759249ef1de1da72f6f7e20b89d4a1a527764"

	st := stabilityai.NewStabilityAIWithClient(&conf, &http.Client{Timeout: 60 * time.Second})
	for {
		res, err := st.ImageToVideoResult(context.TODO(), id)
		if err != nil {
			t.Fatal(err)
		}

		if res.Video != "" {
			filepath, err := res.SaveToLocalFiles(context.TODO(), "/tmp")
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("saved as %s", filepath)
			break
		}

		if res.Status == "in-progress" {
			t.Log("in progress")
			time.Sleep(5 * time.Second)
			continue
		}

		t.Log("unknown status")
		time.Sleep(5 * time.Second)
	}
}
