package leap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"gopkg.in/resty.v1"
)

// 预训练模型 https://docs.tryleap.ai/reference/pre-trained-models
const (
	ModelStableDiffusion_1_5  = "8b1b897c-d66d-45a6-b8d7-8e32421d02cf"
	ModelStableDiffusion_2_1  = "ee88d150-4259-4b77-9d0f-090abe29f650"
	ModelOpenJourney_v4       = "1e7737d7-545e-469f-857f-e4b46eaa151d"
	ModelOpenJourney_v2       = "d66b1686-5e5d-43b2-a2e7-d295d679917c"
	ModelOpenJourney_v1       = "7575ea52-3d4f-400f-9ded-09f7b1b1a5b8"
	ModelModernDisney         = "8ead1e66-5722-4ff6-a13f-b5212f575321"
	ModelFutureDiffusion      = "1285ded4-b11b-4993-a491-d87cdfe6310c"
	ModelRealisticVision_v2_0 = "eab32df0-de26-4b83-a908-a83f3015e971"
	ModelRealisticVision_v4_0 = "37d42ae9-5f5f-4399-b60b-014d35e762a5"
)

// https://docs.tryleap.ai/
type LeapAI struct {
	conf   *config.Config
	client *http.Client
	resty  *resty.Client
}

func NewLeapAI(resolver infra.Resolver, conf *config.Config) *LeapAI {
	client := &http.Client{Timeout: 180 * time.Second}
	restyClient := misc.RestyClient(2).SetTimeout(180 * time.Second)

	if conf.SupportProxy() && conf.LeapAIAutoProxy {
		resolver.MustResolve(func(pp *proxy.Proxy) {
			transport := pp.BuildTransport()
			client.Transport = transport
			restyClient.SetTransport(transport)
		})
	}

	return &LeapAI{conf: conf, client: client, resty: restyClient}
}

func NewLeapAIWithClient(conf *config.Config, client *http.Client, restyClient *resty.Client) *LeapAI {
	return &LeapAI{conf: conf, client: client, resty: restyClient}
}

type TextToImageRequest struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negativePrompt,omitempty"`
	// Version The version of the model to use for the inference. If not provided will default to latest.
	Version string `json:"version,omitempty"`
	// Steps The number of steps to use for the inference.
	Steps int64 `json:"steps"`
	// Width The width of the image to generate.
	Width int64 `json:"width"`
	// Height The height of the image to generate.
	Height int64 `json:"height"`
	// NumberOfImages The number of images to generate for the inference. Max batch size is 20.
	NumberOfImages int64 `json:"numberOfImages,omitempty"`
	// PromptStrength The higher the prompt strength, the closer the generated image will be to the prompt. Must be between 0 and 30.
	PromptStrength int64 `json:"promptStrength,omitempty"`
	// Seed The seed to use for the inference. Must be a positive integer.
	Seed int64 `json:"seed,omitempty"`
	// EnhancePrompt Optionally enhance your prompts automatically to generate better results.
	EnhancePrompt bool `json:"enhancePrompt,omitempty"`
	// UpscaleBy Optionally upscale the generated images. This will make the images look more realistic. The default is x1, which means no upscaling. The maximum is x4.
	// Value: x1 x2 x4
	UpscaleBy string `json:"upscaleBy,omitempty"`
	// Sampler Choose the sampler used for your inference.
	// ddim, dpm_2a, dpm_plusplus_sde, euler, euler_a, unipc
	Sampler string `json:"sampler,omitempty"`
	// RestoreFaces Optionally apply face restoration to the generated images.
	// This will make images of faces look more realistic.
	RestoreFaces bool `json:"restoreFaces,omitempty"`
}

// {
// 	"id": "09dc9d6b-66db-4646-9609-06e86cbb8fa1",
// 	"state": "finished",
// 	"prompt": "A photo of an astronaut riding a horse",
// 	"negativePrompt": "asymmetric, watermarks",
// 	"seed": 4523184,
// 	"width": 512,
// 	"height": 512,
// 	"numberOfImages": 1,
// 	"steps": 50,
// 	"weightsId": "1e7737d7-545e-469f-857f-e4b46eaa151d",
// 	"workspaceId": "017362e8-1427-441d-91b4-1f222ee8ab89",
// 	"createdAt": "2023-06-20T07:03:46.438775+00:00",
// 	"promptStrength": 7,
// 	"images": [
// 	  {
// 		"id": "5617d1cf-e4dd-4256-9c97-0548053bc64a",
// 		"uri": "https://static.tryleap.ai/image-gen-09dc9d6b-66db-4646-9609-06e86cbb8fa1/generated_images/0.png",
// 		"createdAt": "2023-06-20 07:04:01.634307+00"
// 	  }
// 	],
// 	"modelId": "1e7737d7-545e-469f-857f-e4b46eaa151d",
// 	"upscalingOption": "x1",
// 	"sampler": "ddim",
// 	"isDeleted": false,
// 	"routedToQueue": "inference"
//   }

type TextToImageResponse struct {
	ID string `json:"id"`
	// State: queued,finished,processing
	State  string  `json:"state"`
	Images []Image `json:"images"`
}

func (resp *TextToImageResponse) GetID() string {
	return resp.ID
}

func (resp *TextToImageResponse) GetState() string {
	return resp.State
}

func (resp *TextToImageResponse) IsFinished() bool {
	return resp.State == StateFinished
}

func (resp *TextToImageResponse) IsProcessing() bool {
	return resp.State == StateProcessing || resp.State == StateQueued
}

const (
	StateQueued     = "queued"
	StateFinished   = "finished"
	StateProcessing = "processing"
)

func (resp *TextToImageResponse) GetImages() []string {
	var images []string
	for _, img := range resp.Images {
		images = append(images, img.URI)
	}

	return images
}

func (resp *TextToImageResponse) UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) ([]string, error) {
	var resources []string
	for _, img := range resp.Images {
		ret, err := up.UploadRemoteFile(ctx, img.URI, int(uid), uploader.DefaultUploadExpireAfterDays, "png", false)
		if err != nil {
			return nil, fmt.Errorf("upload image to qiniu failed: %w", err)
		}

		resources = append(resources, ret)
	}

	return resources, nil
}

type Image struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}

func (leap *LeapAI) TextToImage(ctx context.Context, model string, param *TextToImageRequest) (*TextToImageResponse, error) {
	url := fmt.Sprintf("%s/api/v1/images/models/%s/inferences", leap.conf.LeapAIServers[0], model)

	payload, err := json.Marshal(param)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")
	req.Header.Add("authorization", "Bearer "+leap.conf.LeapAIKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		log.WithFields(log.Fields{
			"status": res.Status,
			"body":   string(body),
		}).Errorf("request failed: %s", res.Status)
		return nil, fmt.Errorf("request failed: %s", res.Status)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resp TextToImageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (leap *LeapAI) QueryTextToImageJobResult(ctx context.Context, model string, inferenceId string) (*TextToImageResponse, error) {
	url := fmt.Sprintf("%s/api/v1/images/models/%s/inferences/%s", leap.conf.LeapAIServers[0], model, inferenceId)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")
	req.Header.Add("authorization", "Bearer "+leap.conf.LeapAIKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		log.WithFields(log.Fields{
			"status": res.Status,
			"body":   string(body),
		}).Errorf("request failed: %s", res.Status)
		return nil, fmt.Errorf("request failed: %s", res.Status)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// log.WithFields(log.Fields{"body": string(body)}).Debugf("leap query job result")

	var resp TextToImageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

type RemixImageRequest struct {
	ImageUrl       string `json:"imageUrl,omitempty"`
	Files          string `json:"files,omitempty"`
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negativePrompt,omitempty"`
	// Version The version of the model to use for the inference. If not provided will default to latest.
	Version string `json:"version,omitempty"`
	// Steps The number of steps to use for the inference.
	Steps int64 `json:"steps,omitempty"`

	// NumberOfImages The number of images to generate for the inference. Max batch size is 20.
	NumberOfImages int64 `json:"numberOfImages,omitempty"`

	// Seed The seed to use for the inference. Must be a positive integer.
	Seed int64 `json:"seed,omitempty"`
	// The segmentation mode that should be used when generating the image.
	// 可选值：canny, mlsd, pose, scribble
	// canny: Canny mode is the default mode. It is good for generating images that have complex shapes and outlines.
	// mlsd: M-LSD excels at generating images that have a lot of straight lines, such as buildings.
	// pose: Pose is good for generating consistent poses, for either humans or non-human characters.
	// scribble: Scribble is great at converting a rough sketch into a finished image.
	Mode string `json:"mode,omitempty"`
}

type RemixImageResponse struct {
	ID             string  `json:"id"`
	SourceImageUri string  `json:"sourceImageUri"`
	Status         string  `json:"status"`
	Images         []Image `json:"images"`
}

func (resp *RemixImageResponse) GetID() string {
	return resp.ID
}
func (resp *RemixImageResponse) GetState() string {
	return resp.Status
}

func (resp *RemixImageResponse) IsFinished() bool {
	return resp.Status == StateFinished
}

func (resp *RemixImageResponse) IsProcessing() bool {
	return resp.Status == StateProcessing || resp.Status == StateQueued
}

func (resp *RemixImageResponse) GetImages() []string {
	var images []string
	for _, img := range resp.Images {
		images = append(images, img.URI)
	}

	return images
}

func (resp *RemixImageResponse) UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) ([]string, error) {
	var resources []string
	for _, img := range resp.Images {
		ret, err := up.UploadRemoteFile(ctx, img.URI, int(uid), uploader.DefaultUploadExpireAfterDays, "png", false)
		if err != nil {
			return nil, fmt.Errorf("upload image to qiniu failed: %w", err)
		}

		resources = append(resources, ret)
	}

	return resources, nil
}

// RemixImageUpload remix image by upload file
func (leap *LeapAI) RemixImageUpload(ctx context.Context, model string, param *RemixImageRequest) (*RemixImageResponse, error) {
	url := fmt.Sprintf("%s/api/v1/images/models/%s/remix", leap.conf.LeapAIServers[0], model)

	formData := map[string]string{
		"prompt": param.Prompt,
		"seed":   strconv.Itoa(int(param.Seed)),
	}

	if param.NegativePrompt != "" {
		formData["negativePrompt"] = param.NegativePrompt
	}

	if param.NumberOfImages > 0 {
		formData["numberOfImages"] = strconv.Itoa(int(param.NumberOfImages))
	}

	if param.Mode != "" {
		formData["mode"] = param.Mode
	}

	if param.Steps > 0 {
		formData["steps"] = strconv.Itoa(int(param.Steps))
	}

	data, err := os.ReadFile(param.Files)
	if err != nil {
		return nil, err
	}
	res, err := leap.resty.R().
		SetFileReader("files", filepath.Base(param.Files), bytes.NewReader(data)).
		SetFormData(formData).
		SetHeader("Authorization", "Bearer "+leap.conf.LeapAIKey).
		SetHeader("Accept", "application/json").
		Post(url)

	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK && res.StatusCode() != http.StatusCreated {
		log.WithFields(log.Fields{
			"status": res.Status,
			"body":   string(res.Body()),
		}).Errorf("request failed: %s", res.Status)
		return nil, fmt.Errorf("request failed: %s", res.Status())
	}

	var resp RemixImageResponse
	if err := json.Unmarshal(res.Body(), &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// RemixImageURL remix image
func (leap *LeapAI) RemixImageURL(ctx context.Context, model string, param *RemixImageRequest) (*RemixImageResponse, error) {
	url := fmt.Sprintf("%s/api/v1/images/models/%s/remix/url", leap.conf.LeapAIServers[0], model)

	payload, err := json.Marshal(param)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")
	req.Header.Add("authorization", "Bearer "+leap.conf.LeapAIKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		log.WithFields(log.Fields{
			"status": res.Status,
			"body":   string(body),
		}).Errorf("request failed: %s", res.Status)
		return nil, fmt.Errorf("request failed: %s", res.Status)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resp RemixImageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (leap *LeapAI) QueryRemixImageJobResult(ctx context.Context, model string, remixId string) (*RemixImageResponse, error) {
	url := fmt.Sprintf("%s/api/v1/images/models/%s/remix/%s", leap.conf.LeapAIServers[0], model, remixId)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")
	req.Header.Add("authorization", "Bearer "+leap.conf.LeapAIKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		log.WithFields(log.Fields{
			"status": res.Status,
			"body":   string(body),
		}).Errorf("request failed: %s", res.Status)
		return nil, fmt.Errorf("request failed: %s", res.Status)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// log.WithFields(log.Fields{"body": string(body)}).Debugf("leap query job result")

	var resp RemixImageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
