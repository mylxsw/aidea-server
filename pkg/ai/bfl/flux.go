package bfl

import (
	"context"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/glacier/infra"
	"gopkg.in/resty.v1"
	"time"
)

type Flux struct {
	conf  *config.Config
	resty *resty.Client
}

func NewFlux(conf *config.Config, resolver infra.Resolver) *Flux {
	restyClient := misc.RestyClient(2).SetTimeout(180 * time.Second)

	if conf.SupportProxy() && conf.GetimgAIAutoProxy {
		resolver.MustResolve(func(pp *proxy.Proxy) {
			restyClient.SetTransport(pp.BuildTransport())
		})
	}

	return &Flux{conf: conf, resty: restyClient}
}

// TextToImageRequest Text to Image request
// Docs: https://api.bfl.ml/scalar#tag/tasks/POST/v1/flux-pro
type TextToImageRequest struct {
	Prompt           string  `json:"prompt"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	Steps            int     `json:"steps,omitempty"`
	PromptUnsampling bool    `json:"prompt_unsampling,omitempty"`
	Seed             int     `json:"seed,omitempty"`
	Guidance         float64 `json:"guidance,omitempty"`
	SafetyTolerance  int     `json:"safety_tolerance,omitempty"`
	Interval         int     `json:"interval,omitempty"`
	// OutputFormat Output format for the generated image. Can be 'jpeg' or 'png'.
	OutputFormat string `json:"output_format,omitempty"`
}

type TextToImageResponse struct {
	ID string `json:"id"`
}

func (f *Flux) TextToImage(ctx context.Context, req TextToImageRequest) (*TextToImageResponse, error) {
	return nil, nil
}
