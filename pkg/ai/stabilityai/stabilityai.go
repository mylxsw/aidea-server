package stabilityai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/must"
)

const (
	UpscaleEsganV1X2PlusModel                   = "esrgan-v1-x2plus"
	UpscaleStableDiffusionX4LatentUpscalerModel = "stable-diffusion-x4-latent-upscaler"
)

type StabilityAI struct {
	conf   *config.Config
	client *http.Client
}

func NewStabilityAI(resolver infra.Resolver, conf *config.Config) *StabilityAI {
	client := &http.Client{Timeout: 300 * time.Second}
	if conf.SupportProxy() && conf.StabilityAIAutoProxy {
		resolver.MustResolve(func(pp *proxy.Proxy) {
			client.Transport = pp.BuildTransport()
		})
	}

	return &StabilityAI{conf: conf, client: client}
}

func NewStabilityAIWithClient(conf *config.Config, client *http.Client) *StabilityAI {
	return &StabilityAI{conf: conf, client: client}
}

type BalanceResponse struct {
	Credits float64 `json:"credits"`
}

// AccountBalance 获取账户余额, $10 = 1000 credits
func (ai *StabilityAI) AccountBalance(ctx context.Context) (float64, error) {
	// Build the request
	req, _ := http.NewRequest("GET", ai.conf.StabilityAIServer[0]+"/v1/user/balance", nil)
	req.Header.Add("Authorization", "Bearer "+ai.conf.StabilityAIKey)
	if ai.conf.StabilityAIOrganization != "" {
		req.Header.Add("Organization", ai.conf.StabilityAIOrganization)
	}

	// Execute the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0.0, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body map[string]interface{}
		if err := json.Unmarshal(must.Must(io.ReadAll(resp.Body)), &body); err != nil {
			return 0.0, fmt.Errorf("failed to decode response body: %v", err)
		}

		return 0.0, fmt.Errorf("请求失败： %s", body["message"])
	}

	var ret BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return 0.0, fmt.Errorf("failed to decode response body: %v", err)
	}

	return ret.Credits, nil
}

type ErrorResponse struct {
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
	Name    string `json:"name,omitempty"`
}

// ImageToImage 图片转图片 https://platform.stability.ai/docs/features/image-to-image?tab=python
// 注意：输入图片的宽度和高度值必须为 64 的整数倍
func (ai *StabilityAI) ImageToImage(ctx context.Context, model string, param ImageToImageRequest) (*TextToImageResponse, error) {
	data := &bytes.Buffer{}
	writer := multipart.NewWriter(data)

	// Write the init image to the request
	initImageWriter, _ := writer.CreateFormField("init_image")
	initImageFile, initImageErr := os.Open(param.InitImage)
	if initImageErr != nil {
		writer.Close()
		return nil, initImageErr
	}

	_, _ = io.Copy(initImageWriter, initImageFile)

	_ = writer.WriteField("text_prompts[0][text]", param.TextPrompt)
	_ = writer.WriteField("init_image_mode", param.InitImageMode)

	if param.ImageStrength > 0 && param.InitImageMode == "IMAGE_STRENGTH" {
		_ = writer.WriteField("image_strength", fmt.Sprintf("%.2f", param.ImageStrength))
	}

	if param.StepScheduleStart > 0 && param.InitImageMode == "STEP_SCHEDULE" {
		_ = writer.WriteField("step_schedule_start", fmt.Sprintf("%.2f", param.StepScheduleStart))
	}

	if param.StepScheduleEnd > 0 && param.InitImageMode == "STEP_SCHEDULE" {
		_ = writer.WriteField("step_schedule_end", fmt.Sprintf("%.2f", param.StepScheduleEnd))
	}

	if param.CfgScale > 0 {
		_ = writer.WriteField("cfg_scale", strconv.Itoa(param.CfgScale))
	}
	// _ = writer.WriteField("clip_guidance_preset", "FAST_BLUE")
	if param.Samples > 0 {
		_ = writer.WriteField("samples", strconv.Itoa(param.Samples))
	}
	if param.Steps > 0 {
		_ = writer.WriteField("steps", strconv.Itoa(param.Steps))
	}
	_ = writer.WriteField("seed", strconv.Itoa(param.Seed))
	if param.StylePreset != "" {
		_ = writer.WriteField("style_preset", param.StylePreset)
	}

	writer.Close()

	// Execute the request
	payload := bytes.NewReader(data.Bytes())
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/generation/%s/image-to-image", ai.conf.StabilityAIServer[0], model), payload)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+ai.conf.StabilityAIKey)
	if ai.conf.StabilityAIOrganization != "" {
		req.Header.Add("Organization", ai.conf.StabilityAIOrganization)
	}

	resp, err := ai.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody := must.Must(io.ReadAll(resp.Body))
		log.F(log.M{
			"status_code": resp.StatusCode,
			"status":      resp.Status,
			"body":        string(respBody),
		}).Errorf("failed to decode response body: %v", err)

		var body map[string]interface{}
		if err := json.Unmarshal(respBody, &body); err != nil {
			log.Errorf("failed to decode response body: %v", err)
			return nil, errors.New(string(respBody))
		}

		return nil, fmt.Errorf("请求失败: %s", body["message"])
	}

	var body TextToImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return &body, nil
}

// Upscale 图片放大， width 和 height 只有一个值会生效，这里取最大值
// https://platform.stability.ai/docs/features/image-upscaling?tab=python
// Upscaler	                            Resolution	       Credit   Cost
// esrgan-v1-x2plus	                    Any	               0.2      ($0.002)
// stable-diffusion-x4-latent-upscaler	512 x 512	       8        ($0.08)
// stable-diffusion-x4-latent-upscaler	Above 512 x 512	   12       ($0.12)
func (ai *StabilityAI) Upscale(ctx context.Context, model string, imagePath string, width int64, height int64) (*TextToImageResponse, error) {
	data := &bytes.Buffer{}
	writer := multipart.NewWriter(data)

	// Write the init image to the request
	initImageWriter, _ := writer.CreateFormField("image")
	initImageFile, initImageErr := os.Open(imagePath)
	if initImageErr != nil {
		writer.Close()
		return nil, initImageErr
	}

	_, _ = io.Copy(initImageWriter, initImageFile)

	// Write the options to the request
	if width > height {
		_ = writer.WriteField("width", fmt.Sprintf("%d", width))
	} else {
		_ = writer.WriteField("height", fmt.Sprintf("%d", height))
	}

	writer.Close()

	// Execute the request
	payload := bytes.NewReader(data.Bytes())
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/generation/%s/image-to-image/upscale", ai.conf.StabilityAIServer[0], model), payload)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+ai.conf.StabilityAIKey)
	if ai.conf.StabilityAIOrganization != "" {
		req.Header.Add("Organization", ai.conf.StabilityAIOrganization)
	}

	resp, err := ai.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody := must.Must(io.ReadAll(resp.Body))
		log.F(log.M{
			"status_code": resp.StatusCode,
			"status":      resp.Status,
			"body":        string(respBody),
		}).Errorf("failed to decode response body: %v", err)

		var body map[string]interface{}
		if err := json.Unmarshal(respBody, &body); err != nil {
			log.Errorf("failed to decode response body: %v", err)
			return nil, errors.New(string(respBody))
		}

		return nil, fmt.Errorf("请求失败： %s", body["message"])
	}

	var body TextToImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return &body, nil

}

// TextToImage 文本转图片 https://platform.stability.ai/rest-api#tag/v1generation/operation/textToImage
func (ai *StabilityAI) TextToImage(model string, param TextToImageRequest) (*TextToImageResponse, error) {
	reqData, err := json.Marshal(param)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %v", err)
	}

	log.Debugf("request data: %s", string(reqData))

	client := misc.RestyClient(2).R().
		SetHeader("Authorization", "Bearer "+ai.conf.StabilityAIKey).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json")

	if ai.conf.StabilityAIOrganization != "" {
		client.SetHeader("Organization", ai.conf.StabilityAIOrganization)
	}

	resp, err := client.SetBody(reqData).Post(fmt.Sprintf("%s/v1/generation/%s/text-to-image", ai.conf.StabilityAIServer[0], model))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	if resp.IsError() {
		return nil, errorHandle(resp.Body())
	}

	var body TextToImageResponse
	if err := json.Unmarshal(resp.Body(), &body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return &body, nil
}

func errorHandle(body []byte) error {
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("请求失败: %s", string(body))
	}

	switch errResp.Name {
	case "invalid_prompts":
		return errors.New("检测到违规内容，请修改后重试")
	}

	return fmt.Errorf("请求失败: %s", errResp.Message)
}

type TextToImageImage struct {
	Base64       string `json:"base64"`
	Seed         uint32 `json:"seed"`
	FinishReason string `json:"finishReason"`
}

type TextToImageResponse struct {
	Images []TextToImageImage `json:"artifacts"`
}

func (resp *TextToImageResponse) SaveToLocalFiles(ctx context.Context, savePath string) ([]string, error) {
	var resources []string
	for _, img := range resp.Images {
		data, err := base64.StdEncoding.DecodeString(img.Base64)
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

func (resp *TextToImageResponse) UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) ([]string, error) {
	var resources []string
	for _, img := range resp.Images {
		data, err := base64.StdEncoding.DecodeString(img.Base64)
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

type TextPrompts struct {
	Text   string  `json:"text"`
	Weight float64 `json:"weight"`
}

type ImageToImageRequest struct {
	// TextPrompts An array of text prompts to use for generation
	TextPrompt string `json:"text_prompt,omitempty"`

	// InitImage Image used to initialize the diffusion process, in lieu of random noise.
	InitImage string `json:"init_image,omitempty"`

	// InitImageMode Whether to use image_strength or step_schedule_* to control how much influence the init_image has on the result.
	// 可选值：IMAGE_STRENGTH 或者 STEP_SCHEDULE
	InitImageMode string `json:"init_image_mode,omitempty"`

	// 当 InitImageMode = IMAGE_STRENGTH 时

	// ImageStrength How much influence the init_image has on the diffusion process.
	// Values close to 1 will yield images very similar to the init_image while values close to 0 will
	// yield images wildly different than the init_image.
	// The behavior of this is meant to mirror DreamStudio's "Image Strength" slider.
	//
	// This parameter is just an alternate way to set step_schedule_start,
	// which is done via the calculation 1 - image_strength.
	// For example, passing in an Image Strength of 35% (0.35) would result in a step_schedule_start of 0.65
	ImageStrength float64 `json:"image_strength,omitempty"`

	// 当 InitImageMode = STEP_SCHEDULE 时

	// StepScheduleStart The starting value for the step schedule.
	// Skips a proportion of the start of the diffusion steps, allowing the init_image to influence the final generated image.
	// Lower values will result in more influence from the init_image, while higher values will result in more influence from the diffusion steps.
	// (e.g. a value of 0 would simply return you the init_image, where a value of 1 would return you a completely different image.)
	StepScheduleStart float64 `json:"step_schedule_start,omitempty"`

	// StepScheduleEnd The ending value for the step schedule.
	// Skips a proportion of the end of the diffusion steps, allowing the init_image to influence the final generated image.
	// Lower values will result in more influence from the init_image, while higher values will result in more influence from the diffusion steps.
	StepScheduleEnd float64 `json:"step_schedule_end,omitempty"`

	// CfgScale How strictly the diffusion process adheres to the prompt text (higher values keep your image closer to your prompt)
	// number (CfgScale) [ 0 .. 35 ]
	// Default: 7
	CfgScale int `json:"cfg_scale,omitempty"`

	// ClipGuidancePreset
	// string (ClipGuidancePreset)
	// Default: NONE
	// Enum: FAST_BLUE FAST_GREEN NONE SIMPLE SLOW SLOWER SLOWEST
	ClipGuidancePreset string `json:"clip_guidance_preset,omitempty"`

	// Sampler Which sampler to use for the diffusion process. If this value is omitted we'll automatically select an appropriate sampler for you.
	// DDIM DDPM K_DPMPP_2M K_DPMPP_2S_ANCESTRAL K_DPM_2 K_DPM_2_ANCESTRAL K_EULER K_EULER_ANCESTRAL K_HEUN K_LMS
	Sampler string `json:"sampler,omitempty"`

	// Samples Number of images to generate
	// integer (Samples) [ 1 .. 10 ]
	// Default: 1
	Samples int `json:"samples,omitempty"`

	// Seed Random noise seed (omit this option or use 0 for a random seed)
	// integer (Seed) [ 0 .. 4294967295 ]
	// Default: 0
	Seed int `json:"seed,omitempty"`

	// Steps Number of diffusion steps to run
	// integer (Steps) [ 10 .. 150 ]
	// Default: 50
	Steps int `json:"steps,omitempty"`

	// StylePreset Pass in a style preset to guide the image model towards a particular style. This list of style presets is subject to change.
	// string (StylePreset)
	// Enum: 3d-model analog-film anime cinematic comic-book digital-art enhance fantasy-art isometric line-art low-poly modeling-compound neon-punk origami photographic pixel-art tile-texture
	StylePreset string `json:"style_preset,omitempty"`
}

type TextToImageRequest struct {
	// TextPrompts An array of text prompts to use for generation
	TextPrompts []TextPrompts `json:"text_prompts,omitempty"`

	// Width of the image in pixels.
	// integer (DiffuseImageHeight) multiple of 64 >= 128
	// Default: 512
	// Must be in increments of 64 and pass the following validation:
	//     For 768 engines: 589,824 ≤ height * width ≤ 1,048,576
	//     All other engines: 262,144 ≤ height * width ≤ 1,048,576
	Width int `json:"width,omitempty"`
	// Height of the image in pixels.
	// integer (DiffuseImageHeight) multiple of 64 >= 128
	// Default: 512
	// Must be in increments of 64 and pass the following validation:
	//     For 768 engines: 589,824 ≤ height * width ≤ 1,048,576
	//     All other engines: 262,144 ≤ height * width ≤ 1,048,576
	Height int `json:"height,omitempty"`

	// CfgScale How strictly the diffusion process adheres to the prompt text (higher values keep your image closer to your prompt)
	// number (CfgScale) [ 0 .. 35 ]
	// Default: 7
	CfgScale int `json:"cfg_scale,omitempty"`

	// ClipGuidancePreset
	// string (ClipGuidancePreset)
	// Default: NONE
	// Enum: FAST_BLUE FAST_GREEN NONE SIMPLE SLOW SLOWER SLOWEST
	ClipGuidancePreset string `json:"clip_guidance_preset,omitempty"`

	// Sampler Which sampler to use for the diffusion process. If this value is omitted we'll automatically select an appropriate sampler for you.
	// DDIM DDPM K_DPMPP_2M K_DPMPP_2S_ANCESTRAL K_DPM_2 K_DPM_2_ANCESTRAL K_EULER K_EULER_ANCESTRAL K_HEUN K_LMS
	Sampler string `json:"sampler,omitempty"`

	// Samples Number of images to generate
	// integer (Samples) [ 1 .. 10 ]
	// Default: 1
	Samples int `json:"samples,omitempty"`

	// Seed Random noise seed (omit this option or use 0 for a random seed)
	// integer (Seed) [ 0 .. 4294967295 ]
	// Default: 0
	Seed int `json:"seed,omitempty"`

	// Steps Number of diffusion steps to run
	// integer (Steps) [ 10 .. 150 ]
	// Default: 50
	Steps int `json:"steps,omitempty"`

	// StylePreset Pass in a style preset to guide the image model towards a particular style. This list of style presets is subject to change.
	// string (StylePreset)
	// Enum: 3d-model analog-film anime cinematic comic-book digital-art enhance fantasy-art isometric line-art low-poly modeling-compound neon-punk origami photographic pixel-art tile-texture
	StylePreset string `json:"style_preset,omitempty"`
}
