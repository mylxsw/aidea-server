package stabilityai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/maps"
	"github.com/mylxsw/go-utils/must"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
)

type TextToImageImageV3 struct {
	Image        string `json:"image"`
	Seed         uint32 `json:"seed"`
	FinishReason string `json:"finish_reason"`
}

func (res TextToImageImageV3) ToImageResponse() *TextToImageResponse {
	return &TextToImageResponse{
		Images: []TextToImageImage{
			{
				Base64:       res.Image,
				Seed:         res.Seed,
				FinishReason: res.FinishReason,
			},
		},
	}
}

func (ai *StabilityAI) ImageCoreGenerate(req TextToImageRequest) (*TextToImageImageV3, error) {
	client := misc.RestyClient(2).R().
		SetHeader("Authorization", "Bearer "+ai.conf.StabilityAIKey).
		SetHeader("Accept", "application/json;type=image/png")

	if ai.conf.StabilityAIOrganization != "" {
		client.SetHeader("Organization", ai.conf.StabilityAIOrganization)
	}

	data := map[string]string{
		"prompt": array.Reduce(
			array.Filter(req.TextPrompts, func(item TextPrompts, _ int) bool { return item.Weight >= 0 }),
			func(carry string, item TextPrompts) string {
				return carry + "," + item.Text
			},
			"",
		),
		"aspect_ratio":  misc.ResolveAspectRatio(req.Width, req.Height),
		"output_format": "png",
	}

	negativePrompts := array.Filter(req.TextPrompts, func(item TextPrompts, _ int) bool { return item.Weight < 0 })
	if len(negativePrompts) > 0 {
		data["negative_prompt"] = array.Reduce(
			negativePrompts,
			func(carry string, item TextPrompts) string {
				return carry + "," + item.Text
			},
			"",
		)
	}

	if req.Seed != 0 {
		data["seed"] = fmt.Sprintf("%d", req.Seed)
	}

	if req.StylePreset != "" {
		data["style_preset"] = req.StylePreset
	}

	formData, contentType, err := createFormData(maps.Values(maps.Map(data, func(value string, key string) func(writer *multipart.Writer) error {
		return func(writer *multipart.Writer) error {
			return writer.WriteField(key, value)
		}
	})))

	resp, err := client.SetHeader("Content-Type", contentType).SetBody(formData).Post(fmt.Sprintf("%s/v2beta/stable-image/generate/core", ai.conf.StabilityAIServer[0]))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	if resp.IsError() {
		return nil, errorHandle(resp.Body())
	}

	var body TextToImageImageV3
	if err := json.Unmarshal(resp.Body(), &body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return &body, nil
}

func (ai *StabilityAI) StableDiffusionV3TextToImage(model string, req TextToImageRequest) (*TextToImageImageV3, error) {
	client := misc.RestyClient(2).R().
		SetHeader("Authorization", "Bearer "+ai.conf.StabilityAIKey).
		SetHeader("Accept", "application/json;type=image/png")

	if ai.conf.StabilityAIOrganization != "" {
		client.SetHeader("Organization", ai.conf.StabilityAIOrganization)
	}

	data := map[string]string{
		"prompt": array.Reduce(
			array.Filter(req.TextPrompts, func(item TextPrompts, _ int) bool { return item.Weight >= 0 }),
			func(carry string, item TextPrompts) string {
				return carry + "," + item.Text
			},
			"",
		),
		"aspect_ratio":  misc.ResolveAspectRatio(req.Width, req.Height),
		"output_format": "png",
		"model":         model,
		"mode":          "text-to-image",
	}

	negativePrompts := array.Filter(req.TextPrompts, func(item TextPrompts, _ int) bool { return item.Weight < 0 })
	if len(negativePrompts) > 0 {
		data["negative_prompt"] = array.Reduce(
			negativePrompts,
			func(carry string, item TextPrompts) string {
				return carry + "," + item.Text
			},
			"",
		)
	}

	if req.Seed != 0 {
		data["seed"] = fmt.Sprintf("%d", req.Seed)
	}

	if req.StylePreset != "" {
		data["style_preset"] = req.StylePreset
	}

	formData, contentType, err := createFormData(maps.Values(maps.Map(data, func(value string, key string) func(writer *multipart.Writer) error {
		return func(writer *multipart.Writer) error {
			return writer.WriteField(key, value)
		}
	})))

	resp, err := client.SetHeader("Content-Type", contentType).SetBody(formData).Post(fmt.Sprintf("%s/v2beta/stable-image/generate/core", ai.conf.StabilityAIServer[0]))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	if resp.IsError() {
		return nil, errorHandle(resp.Body())
	}

	var body TextToImageImageV3
	if err := json.Unmarshal(resp.Body(), &body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return &body, nil
}

func (ai *StabilityAI) StableDiffusionV3ImageToImage(model string, req ImageToImageRequest) (*TextToImageImageV3, error) {
	data := &bytes.Buffer{}
	writer := multipart.NewWriter(data)

	// Write the init image to the request
	initImageWriter, _ := writer.CreateFormField("image")
	initImageFile, initImageErr := os.Open(req.InitImage)
	if initImageErr != nil {
		writer.Close()
		return nil, initImageErr
	}

	_, _ = io.Copy(initImageWriter, initImageFile)

	_ = writer.WriteField("prompt", req.TextPrompt)
	_ = writer.WriteField("mode", "image-to-image")
	_ = writer.WriteField("model", model)

	if req.NegativePrompt != "" {
		_ = writer.WriteField("negative_prompt", req.NegativePrompt)
	}

	if req.ImageStrength > 0 {
		_ = writer.WriteField("strength", fmt.Sprintf("%.2f", req.ImageStrength))
	}

	_ = writer.WriteField("seed", strconv.Itoa(req.Seed))

	writer.Close()

	// Execute the request
	payload := bytes.NewReader(data.Bytes())
	r, _ := http.NewRequest("POST", fmt.Sprintf("%s/v2beta/stable-image/generate/sd3", ai.conf.StabilityAIServer[0]), payload)
	r.Header.Add("Content-Type", writer.FormDataContentType())
	r.Header.Add("Accept", "application/json")
	r.Header.Add("Authorization", "Bearer "+ai.conf.StabilityAIKey)
	if ai.conf.StabilityAIOrganization != "" {
		r.Header.Add("Organization", ai.conf.StabilityAIOrganization)
	}

	resp, err := ai.client.Do(r)
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

	var body TextToImageImageV3
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return &body, nil
}

func createFormData(fields []func(writer *multipart.Writer) error) (io.Reader, string, error) {
	data := &bytes.Buffer{}
	writer := multipart.NewWriter(data)
	defer writer.Close()

	for _, field := range fields {
		if err := field(writer); err != nil {
			return nil, "", err
		}
	}

	return data, writer.FormDataContentType(), nil
}
