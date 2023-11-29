package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"gopkg.in/resty.v1"
	"math/rand"
	"time"
)

type DalleImageClient struct {
	conf *Config
	http *resty.Client
}

func NewDalleImageClient(conf *Config, pp *proxy.Proxy) *DalleImageClient {
	restyClient := misc.RestyClient(2).SetTimeout(180 * time.Second)
	if pp != nil && conf.AutoProxy {
		restyClient.SetTransport(pp.BuildTransport())
	}

	return &DalleImageClient{conf: conf, http: restyClient}
}

type ImageRequest struct {
	// Prompt A text description of the desired image(s).
	// The maximum length is 1000 characters for dall-e-2 and 4000 characters for dall-e-3
	Prompt string `json:"prompt"`
	// Model The model to use for image generation. Defaults to dall-e-2
	Model string `json:"model,omitempty"`
	// N The number of images to generate. Must be between 1 and 10. For dall-e-3, only n=1 is supported.
	N int64 `json:"n,omitempty"`
	// Quality The quality of the image that will be generated.
	// hd creates images with finer details and greater consistency across the image.
	// This param is only supported for dall-e-3. Defaults to standard
	Quality string `json:"quality,omitempty"`
	// ResponseFormat The format in which the generated images are returned. Must be one of url or b64_json.
	// Defaults to url
	ResponseFormat string `json:"response_format,omitempty"`
	// Size The size of the generated images.
	// Must be one of 256x256, 512x512, or 1024x1024 for dall-e-2.
	// Must be one of 1024x1024, 1792x1024, or 1024x1792 for dall-e-3 models.
	// Defaults to 1024x1024
	Size string `json:"size,omitempty"`
	// Style The style of the generated images.
	// Must be one of vivid or natural.
	// Vivid causes the model to lean towards generating hyper-real and dramatic images.
	// Natural causes the model to produce more natural, less hyper-real looking images.
	// This param is only supported for dall-e-3.
	// Defaults to vivid
	Style string `json:"style,omitempty"`
	// User A unique identifier representing your end-user, which can help OpenAI to monitor and detect abuse
	User string `json:"user,omitempty"`
}

type ImageResponseDataInner struct {
	// Base64JSON The base64-encoded JSON of the generated image, if response_format is b64_json.
	Base64JSON string `json:"b64_json,omitempty"`
	// URL The URL of the generated image, if response_format is url (default).
	URL string `json:"url,omitempty"`
	// RevisedPrompt The prompt that was used to generate the image, if there was any revision to the prompt.
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type ImageResponse struct {
	Created int64                    `json:"created,omitempty"`
	Data    []ImageResponseDataInner `json:"data,omitempty"`
	Error   *ErrorResponseInner      `json:"error,omitempty"`
}

func (resp *ImageResponse) UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) ([]string, error) {
	var resources []string
	for _, img := range resp.Data {
		data, err := base64.StdEncoding.DecodeString(img.Base64JSON)
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

type ErrorResponseInner struct {
	Message string `json:"message,omitempty"`
	Type    string `json:"type,omitempty"`
}

func (client *DalleImageClient) pickAPIKey() string {
	return client.conf.OpenAIKeys[rand.Intn(len(client.conf.OpenAIKeys))]
}

func (client *DalleImageClient) pickServer() string {
	return client.conf.OpenAIServers[rand.Intn(len(client.conf.OpenAIServers))]
}

func (client *DalleImageClient) CreateImage(ctx context.Context, request ImageRequest) (*ImageResponse, error) {
	resp, err := client.http.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+client.pickAPIKey()).
		SetBody(request).
		Post(fmt.Sprintf("%s/images/generations", client.pickServer()))
	if err != nil {
		return nil, err
	}

	var ret ImageResponse
	if err := json.Unmarshal(resp.Body(), &ret); err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, fmt.Errorf("%s: %s", ret.Error.Type, ret.Error.Message)
	}

	return &ret, nil
}
