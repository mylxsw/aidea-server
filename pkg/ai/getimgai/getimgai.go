package getimgai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/must"
	"gopkg.in/resty.v1"
)

type GetimgAI struct {
	conf  *config.Config
	resty *resty.Client
}

func NewGetimgAI(conf *config.Config, resolver infra.Resolver) *GetimgAI {
	restyClient := misc.RestyClient(2).SetTimeout(180 * time.Second)

	if conf.SupportProxy() && conf.GetimgAIAutoProxy {
		resolver.MustResolve(func(pp *proxy.Proxy) {
			restyClient.SetTransport(pp.BuildTransport())
		})
	}

	return &GetimgAI{conf: conf, resty: restyClient}
}

func NewGetimgAIWithResty(conf *config.Config, resty *resty.Client) *GetimgAI {
	return &GetimgAI{conf: conf, resty: resty}
}

type ModelResolution struct {
	Width  int64 `json:"width,omitempty"`
	Height int64 `json:"height,omitempty"`
}

type Model struct {
	ID             string          `json:"id,omitempty"`
	Name           string          `json:"name,omitempty"`
	Family         string          `json:"family,omitempty"`
	Pipelines      []string        `json:"pipelines,omitempty"`
	BaseResolution ModelResolution `json:"base_resolution,omitempty"`
	Price          float64         `json:"price,omitempty"`
	AuthorURL      string          `json:"author_url,omitempty"`
	LicenseURL     string          `json:"license_url,omitempty"`
}

func (getimg *GetimgAI) Models(ctx context.Context, family, pipeline string) ([]Model, error) {
	req := getimg.resty.R().
		SetHeader("Authorization", "Bearer "+getimg.conf.GetimgAIKey).
		SetContext(ctx)
	if family != "" {
		req.SetQueryParam("family", family)
	}

	if pipeline != "" {
		req.SetQueryParam("pipeline", pipeline)
	}

	resp, err := req.Get(getimg.conf.GetimgAIServer + "/v1/models")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, fmt.Errorf("get models failed: %s", string(resp.Body()))
	}

	var models []Model
	if err := json.Unmarshal(resp.Body(), &models); err != nil {
		return nil, err
	}

	return models, nil
}

func (getimg *GetimgAI) AccountBalance(ctx context.Context) (float64, error) {
	resp, err := getimg.resty.R().
		SetHeader("Authorization", "Bearer "+getimg.conf.GetimgAIKey).
		SetContext(ctx).
		Get(getimg.conf.GetimgAIServer + "/v1/account/balance")
	if err != nil {
		return 0, err
	}

	if resp.IsError() {
		return 0, fmt.Errorf("get account balance failed: %s", string(resp.Body()))
	}

	var balance struct {
		Amount float64 `json:"amount"`
	}
	if err := json.Unmarshal(resp.Body(), &balance); err != nil {
		return 0, err
	}

	return balance.Amount, nil
}

type TextToImageRequest struct {
	// Model ID supported by this pipeline and family.
	// Use /v1/models?pipeline=text-to-image&family=stable-diffusion to list all available models
	// Default value is stable-diffusion-v1-5.
	Model string `json:"model,omitempty"`

	// Prompt Text input required to guide the image generation.
	// Maximum length is 2048
	Prompt string `json:"prompt,omitempty"`
	// NegativePrompt Text input that will not guide the image generation.
	// Maximum length is 2048.
	NegativePrompt string `json:"negative_prompt,omitempty"`

	// Width The width of the generated image in pixels. To achieve
	// best results use width specified in model base_resolution.
	// Width needs to be multiple of 64.
	// Minimum value is 256, maximum value is 1024. Default value is 512
	Width int64 `json:"width,omitempty"`
	// Height The height of the generated image in pixels. To achieve
	// best results use height specified in model base_resolution.
	// Height needs to be multiple of 64.
	// Minimum value is 256, maximum value is 1024. Default value is 512.
	Height int64 `json:"height,omitempty"`
	// Steps The number of denoising steps. More steps usually can produce higher quality images, but take more time to generate.
	// Minimum value is 1, maximum value is 100. Default value is 25.
	Steps int64 `json:"steps,omitempty"`

	//	Guidance scale as defined in Classifier-Free Diffusion Guidance.
	// Higer guidance forces the model to better follow the prompt, but result in lower quality output.
	// Minimum value is 0, maximum value is 20. Default value is 7.5.
	Guidance float64 `json:"guidance,omitempty"`

	// Seed Makes generation deterministic. Using the same seed and set of parameters will produce identical image each time.
	// Minimum value is 1, maximum value is 2147483647.
	Seed int64 `json:"seed,omitempty"`
	// Scheduler used to denoise the encoded image latents.
	// Values are euler_a, euler, lms, ddim, dpmsolver++, or pndm. Default value is dpmsolver++.
	Scheduler string `json:"scheduler,omitempty"`

	// OutputFormat File format of the output image
	// Values are jpeg or png. Default value is jpeg.
	OutputFormat string `json:"output_format,omitempty"`
}

type ImageResponse struct {
	Image string `json:"image,omitempty"`
	Seed  int64  `json:"seed,omitempty"`
}

func (resp *ImageResponse) SaveToLocalFiles(savePath string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(resp.Image)
	if err != nil {
		return "", fmt.Errorf("decode base64 failed: %w", err)
	}

	key := filepath.Join(savePath, fmt.Sprintf("%s.%s", must.Must(uuid.GenerateUUID()), "png"))
	if err := os.WriteFile(key, data, os.ModePerm); err != nil {
		return "", fmt.Errorf("write image to file failed: %w", err)
	}

	return key, nil
}

func (resp *ImageResponse) UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) (string, error) {
	data, err := base64.StdEncoding.DecodeString(resp.Image)
	if err != nil {
		return "", fmt.Errorf("decode base64 failed: %w", err)
	}

	ret, err := up.UploadStream(ctx, int(uid), uploader.DefaultUploadExpireAfterDays, data, "png")
	if err != nil {
		return "", fmt.Errorf("upload image to qiniu failed: %w", err)
	}

	return ret, nil
}

type ErrorResponse struct {
	Error Error `json:"error,omitempty"`
}

type Error struct {
	Message string `json:"message,omitempty"`
	Type    string `json:"type,omitempty"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

func (getimg *GetimgAI) TextToImage(ctx context.Context, req TextToImageRequest) (*ImageResponse, error) {
	endpoint := "/v1/stable-diffusion/text-to-image"
	model := GetModelByID(req.Model)
	if model != nil {
		if model.Family == ModelFamilySDXL {
			endpoint = "/v1/stable-diffusion-xl/text-to-image"
		}
	}

	resp, err := getimg.resty.R().
		SetHeader("Authorization", "Bearer "+getimg.conf.GetimgAIKey).
		SetHeader("Content-Type", "application/json").
		SetContext(ctx).
		SetBody(req).
		Post(getimg.conf.GetimgAIServer + endpoint)
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body(), &errResp); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("generate image failed: %s", errResp.Error.Message)
	}

	var imageResp ImageResponse
	if err := json.Unmarshal(resp.Body(), &imageResp); err != nil {
		return nil, err
	}

	return &imageResp, nil
}

type ImageToImageRequest struct {
	// Model ID supported by this pipeline and family.
	// Use /v1/models?pipeline=image-to-image&family=stable-diffusion to list all available models.
	// Default value is stable-diffusion-v1-5
	Model string `json:"model,omitempty"`
	// Prompt Text input required to guide the image generation.
	// Maximum length is 2048
	Prompt string `json:"prompt,omitempty"`
	// NegativePrompt Text input that will not guide the image generation.
	// Maximum length is 2048.
	NegativePrompt string `json:"negative_prompt,omitempty"`

	// Image Base64 encoded image that will be used as the a starting
	// point for the generation.
	// Maximum size in each dimension is 1024px.
	Image string `json:"image,omitempty"`

	// Strength	Indicates how much to transform the reference image.
	// When strength is 1, initial image will be ignored.
	// Technically, strength parameter indicates how much noise add to the image.
	// Minimum value is 0, maximum value is 1. Default value is 0.5.
	Strength float64 `json:"strength,omitempty"`

	// Steps The number of denoising steps. More steps usually can produce higher quality images, but take more time to generate.
	// Minimum value is 1, maximum value is 100. Default value is 25.
	Steps int64 `json:"steps,omitempty"`

	//	Guidance scale as defined in Classifier-Free Diffusion Guidance.
	// Higer guidance forces the model to better follow the prompt, but result in lower quality output.
	// Minimum value is 0, maximum value is 20. Default value is 7.5.
	Guidance float64 `json:"guidance,omitempty"`

	// Seed Makes generation deterministic. Using the same seed and set of parameters will produce identical image each time.
	// Minimum value is 1, maximum value is 2147483647.
	Seed int64 `json:"seed,omitempty"`
	// Scheduler used to denoise the encoded image latents.
	// Values are euler_a, euler, lms, ddim, dpmsolver++, or pndm. Default value is dpmsolver++.
	Scheduler string `json:"scheduler,omitempty"`

	// OutputFormat File format of the output image
	// Values are jpeg or png. Default value is jpeg.
	OutputFormat string `json:"output_format,omitempty"`
}

func (getimg *GetimgAI) ImageToImage(ctx context.Context, req ImageToImageRequest) (*ImageResponse, error) {
	endpoint := "/v1/stable-diffusion/image-to-image"
	model := GetModelByID(req.Model)
	if model != nil {
		if model.Family == ModelFamilySDXL {
			endpoint = "/v1/stable-diffusion-xl/image-to-image"
		}
	}

	resp, err := getimg.resty.R().
		SetHeader("Authorization", "Bearer "+getimg.conf.GetimgAIKey).
		SetHeader("Content-Type", "application/json").
		SetContext(ctx).
		SetBody(req).
		Post(getimg.conf.GetimgAIServer + endpoint)
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body(), &errResp); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("image to image failed: %s", errResp.Error.Message)
	}

	var imageResp ImageResponse
	if err := json.Unmarshal(resp.Body(), &imageResp); err != nil {
		return nil, err
	}

	return &imageResp, nil
}

type ControlNetRequest struct {
	// Model ID supported by this pipeline and family.
	// Use /v1/models?pipeline=controlnet&family=stable-diffusion to list all available models
	// Default value is stable-diffusion-v1-5
	Model string `json:"model,omitempty"`
	// Prompt Text input required to guide the image generation.
	// Maximum length is 2048
	Prompt string `json:"prompt,omitempty"`
	// NegativePrompt Text input that will not guide the image generation.
	// Maximum length is 2048.
	NegativePrompt string `json:"negative_prompt,omitempty"`

	// Controlnet Type of ControlNet conditioning.
	// Values are canny-1.1, softedge-1.1, mlsd-1.1, normal-1.1, depth-1.1, openpose-1.1,
	// openpose-full-1.1, scribble-1.1, lineart-1.1, lineart-anime-1.1, or mediapipeface.
	Controlnet string `json:"controlnet,omitempty"`

	// Width The width of the generated image in pixels. To achieve
	// best results use width specified in model base_resolution.
	// Width needs to be multiple of 64.
	// Minimum value is 256, maximum value is 1024. Default value is 512
	Width int64 `json:"width,omitempty"`
	// Height The height of the generated image in pixels. To achieve
	// best results use height specified in model base_resolution.
	// Height needs to be multiple of 64.
	// Minimum value is 256, maximum value is 1024. Default value is 512.
	Height int64 `json:"height,omitempty"`

	// Image Base64 encoded image that will be used as the a starting
	// point for the generation.
	// Maximum size in each dimension is 1024px.
	Image string `json:"image,omitempty"`

	// Strength	Indicates how much to transform the reference image.
	// When strength is 1, initial image will be ignored.
	// Technically, strength parameter indicates how much noise add to the image.
	// Minimum value is 0, maximum value is 1. Default value is 0.5.
	Strength float64 `json:"strength,omitempty"`

	// Steps The number of denoising steps. More steps usually can produce higher quality images, but take more time to generate.
	// Minimum value is 1, maximum value is 100. Default value is 25.
	Steps int64 `json:"steps,omitempty"`

	//	Guidance scale as defined in Classifier-Free Diffusion Guidance.
	// Higer guidance forces the model to better follow the prompt, but result in lower quality output.
	// Minimum value is 0, maximum value is 20. Default value is 7.5.
	Guidance float64 `json:"guidance,omitempty"`

	// Seed Makes generation deterministic. Using the same seed and set of parameters will produce identical image each time.
	// Minimum value is 1, maximum value is 2147483647.
	Seed int64 `json:"seed,omitempty"`
	// Scheduler used to denoise the encoded image latents.
	// Values are euler_a, euler, lms, ddim, dpmsolver++, or pndm. Default value is dpmsolver++.
	Scheduler string `json:"scheduler,omitempty"`

	// OutputFormat File format of the output image
	// Values are jpeg or png. Default value is jpeg.
	OutputFormat string `json:"output_format,omitempty"`
}

func (getimg *GetimgAI) ControlNet(ctx context.Context, req ControlNetRequest) (*ImageResponse, error) {
	resp, err := getimg.resty.R().
		SetHeader("Authorization", "Bearer "+getimg.conf.GetimgAIKey).
		SetHeader("Content-Type", "application/json").
		SetContext(ctx).
		SetBody(req).
		Post(getimg.conf.GetimgAIServer + "/v1/stable-diffusion/controlnet")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body(), &errResp); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("generate image failed: %s", errResp.Error.Message)
	}

	var imageResp ImageResponse
	if err := json.Unmarshal(resp.Body(), &imageResp); err != nil {
		return nil, err
	}

	return &imageResp, nil
}

type InstructRequest struct {
	// Model ID supported by this pipeline and family.
	// Use /v1/models?pipeline=instruct&family=stable-diffusion to list all available models.
	// Default value is stable-diffusion-v1-5
	Model string `json:"model,omitempty"`
	// Prompt Text input required to guide the image generation.
	// Maximum length is 2048
	Prompt string `json:"prompt,omitempty"`
	// NegativePrompt Text input that will not guide the image generation.
	// Maximum length is 2048.
	NegativePrompt string `json:"negative_prompt,omitempty"`

	// Image Base64 encoded image that will be used as the a starting
	// point for the generation.
	// Maximum size in each dimension is 1024px.
	Image string `json:"image,omitempty"`

	// ImageGuidance Higher image guidance produces images that are closely linked to the source image, usually at the expense of lower quality.
	// Minimum value is 1, maximum value is 5. Default value is 1.5.
	ImageGuidance float64 `json:"image_guidance,omitempty"`

	// Strength	Indicates how much to transform the reference image.
	// When strength is 1, initial image will be ignored.
	// Technically, strength parameter indicates how much noise add to the image.
	// Minimum value is 0, maximum value is 1. Default value is 0.5.
	Strength float64 `json:"strength,omitempty"`

	// Steps The number of denoising steps. More steps usually can produce higher quality images, but take more time to generate.
	// Minimum value is 1, maximum value is 100. Default value is 25.
	Steps int64 `json:"steps,omitempty"`

	//	Guidance scale as defined in Classifier-Free Diffusion Guidance.
	// Higer guidance forces the model to better follow the prompt, but result in lower quality output.
	// Minimum value is 0, maximum value is 20. Default value is 7.5.
	Guidance float64 `json:"guidance,omitempty"`

	// Seed Makes generation deterministic. Using the same seed and set of parameters will produce identical image each time.
	// Minimum value is 1, maximum value is 2147483647.
	Seed int64 `json:"seed,omitempty"`
	// Scheduler used to denoise the encoded image latents.
	// Values are euler_a, euler, lms, ddim, dpmsolver++, or pndm. Default value is dpmsolver++.
	Scheduler string `json:"scheduler,omitempty"`

	// OutputFormat File format of the output image
	// Values are jpeg or png. Default value is jpeg.
	OutputFormat string `json:"output_format,omitempty"`
}

func (getimg *GetimgAI) Instruct(ctx context.Context, req InstructRequest) (*ImageResponse, error) {
	resp, err := getimg.resty.R().
		SetHeader("Authorization", "Bearer "+getimg.conf.GetimgAIKey).
		SetHeader("Content-Type", "application/json").
		SetContext(ctx).
		SetBody(req).
		Post(getimg.conf.GetimgAIServer + "/v1/stable-diffusion/instruct")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body(), &errResp); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("text to image failed: %s", errResp.Error.Message)
	}

	var imageResp ImageResponse
	if err := json.Unmarshal(resp.Body(), &imageResp); err != nil {
		return nil, err
	}

	return &imageResp, nil
}

type UpscaleRequest struct {
	// Model ID supported by this pipeline and family. Use /v1/models?pipeline=enhancement&family=upscale to list all available models.
	// Default value is real-esrgan-4x.
	Model string `json:"model,omitempty"`
	// Image Base64 encoded image that will be upscaled.
	// Maximum size in each dimension is 1024px.
	Image string `json:"image,omitempty"`
	// Scale Scaling factor to apply. Image will be enlarged in all dimensions by the provided scale factor.
	// Minimum value is 4, maximum value is 4. Default value is 4.
	Scale int64 `json:"scale,omitempty"`
	// OutputFormat File format of the output image
	// Values are jpeg or png. Default value is jpeg.
	OutputFormat string `json:"output_format,omitempty"`
}

func (getimg *GetimgAI) Upscale(ctx context.Context, req UpscaleRequest) (*ImageResponse, error) {
	resp, err := getimg.resty.R().
		SetHeader("Authorization", "Bearer "+getimg.conf.GetimgAIKey).
		SetHeader("Content-Type", "application/json").
		SetContext(ctx).
		SetBody(req).
		Post(getimg.conf.GetimgAIServer + "/v1/enhacements/upscale")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body(), &errResp); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("generate image failed: %s", errResp.Error.Message)
	}

	var imageResp ImageResponse
	if err := json.Unmarshal(resp.Body(), &imageResp); err != nil {
		return nil, err
	}

	return &imageResp, nil
}

type FixFaceRequest struct {
	// Model ID supported by this pipeline and family.
	// Use /v1/models?pipeline=enhancement&family=face-fix to list all available models
	// Default value is gfpgan-v1-3.
	Model string `json:"model,omitempty"`
	// Image Base64 encoded image that will be upscaled.
	// Maximum size in each dimension is 1024px.
	Image string `json:"image,omitempty"`
	// OutputFormat File format of the output image
	// Values are jpeg or png. Default value is jpeg.
	OutputFormat string `json:"output_format,omitempty"`
}

func (getimg *GetimgAI) FixFace(ctx context.Context, req FixFaceRequest) (*ImageResponse, error) {
	resp, err := getimg.resty.R().
		SetHeader("Authorization", "Bearer "+getimg.conf.GetimgAIKey).
		SetHeader("Content-Type", "application/json").
		SetContext(ctx).
		SetBody(req).
		Post(getimg.conf.GetimgAIServer + "/v1/enhacements/face-fix")
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body(), &errResp); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("generate image failed: %s", errResp.Error.Message)
	}

	var imageResp ImageResponse
	if err := json.Unmarshal(resp.Body(), &imageResp); err != nil {
		return nil, err
	}

	return &imageResp, nil
}
