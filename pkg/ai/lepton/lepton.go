package lepton

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-uuid"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/must"
	"gopkg.in/resty.v1"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

type Lepton struct {
	apiKeys      []string
	qrServerURLs []string
	resty        *resty.Client
}

func Default(conf *config.Config) *Lepton {
	restyClient := misc.RestyClient(2).SetTimeout(180 * time.Second)

	return &Lepton{
		apiKeys:      conf.LeptonAIKeys,
		qrServerURLs: conf.LeptonAIQRServers,
		resty:        restyClient,
	}
}

func New(resolver infra.Resolver, conf *config.Config) *Lepton {
	restyClient := misc.RestyClient(2).SetTimeout(180 * time.Second)

	if conf.SupportProxy() && conf.LeapAIAutoProxy {
		resolver.MustResolve(func(pp *proxy.Proxy) {
			restyClient.SetTransport(pp.BuildTransport())
		})
	}

	return &Lepton{
		apiKeys:      conf.LeptonAIKeys,
		qrServerURLs: conf.LeptonAIQRServers,
		resty:        restyClient,
	}
}

func (ai *Lepton) client() (serverURL, key string) {
	return ai.qrServerURLs[rand.Intn(len(ai.qrServerURLs))], ai.apiKeys[rand.Intn(len(ai.apiKeys))]
}

type QRImageRequest struct {
	ControlImage      string  `json:"control_image,omitempty"`
	Model             string  `json:"model"`
	Prompt            string  `json:"prompt"`
	NegativePrompt    string  `json:"negative_prompt,omitempty"`
	ControlImageRatio float64 `json:"control_image_ratio,omitempty"`
	ControlWeight     float64 `json:"control_weight,omitempty"`
	GuidanceStart     float64 `json:"guidance_start,omitempty"`
	GuidanceEnd       float64 `json:"guidance_end,omitempty"`
	Seed              int64   `json:"seed,omitempty"`
	Steps             int64   `json:"steps,omitempty"`
	CfgScale          int64   `json:"cfg_scale,omitempty"`
	NumImages         int64   `json:"num_images,omitempty"`
}

type ImageResponse []string

func (resp ImageResponse) UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) ([]string, error) {
	var resources []string
	for _, img := range resp {
		data, err := base64.StdEncoding.DecodeString(img)
		if err != nil {
			return nil, fmt.Errorf("decode base64 failed: %w", err)
		}

		ret, err := up.UploadStream(ctx, int(uid), uploader.DefaultUploadExpireAfterDays, data, "png")
		if err != nil {
			return nil, fmt.Errorf("upload image to qiniu failed: %w", err)
		}

		resources = append(resources, ret)
	}

	return resources, nil
}

func (resp ImageResponse) SaveToLocalFiles(ctx context.Context, savePath string) ([]string, error) {
	var resources []string
	for _, img := range resp {
		data, err := base64.StdEncoding.DecodeString(img)
		if err != nil {
			return nil, fmt.Errorf("decode base64 failed: %w", err)
		}

		key := filepath.Join(savePath, fmt.Sprintf("%s.%s", must.Must(uuid.GenerateUUID()), "png"))
		if err := os.WriteFile(key, data, os.ModePerm); err != nil {
			return nil, fmt.Errorf("write image to file failed: %w", err)
		}

		resources = append(resources, key)
	}

	return resources, nil
}

func (ai *Lepton) ImageGenerate(ctx context.Context, req QRImageRequest) (*ImageResponse, error) {
	server, key := ai.client()
	resp, err := ai.resty.R().
		SetHeader("Authorization", "Bearer "+key).
		SetHeader("Content-Type", "application/json").
		SetContext(ctx).
		SetBody(req).
		Post(server + "/generate")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, fmt.Errorf("generate image failed: [%d] %s", resp.StatusCode(), string(resp.Body()))
	}

	var imageResp ImageResponse
	if err := json.Unmarshal(resp.Body(), &imageResp); err != nil {
		return nil, err
	}

	return &imageResp, nil
}
