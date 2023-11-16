package deepai

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
)

const DefaultServerURL = "https://api.deepai.org"

type DeepAI struct {
	conf   *config.Config
	client *http.Client
}

func NewDeepAIRaw(conf *config.Config) *DeepAI {
	return &DeepAI{conf: conf, client: &http.Client{Timeout: 300 * time.Second}}
}

func NewDeepAI(resolver infra.Resolver, conf *config.Config) *DeepAI {
	client := &http.Client{Timeout: 300 * time.Second}
	if conf.SupportProxy() && conf.DeepAIAutoProxy {
		resolver.MustResolve(func(pp *proxy.Proxy) {
			client.Transport = pp.BuildTransport()
		})
	}

	return &DeepAI{conf: conf, client: client}
}

func (ai *DeepAI) lb() string {
	// TODO load balance
	return strings.TrimRight(ai.conf.DeepAIServer[0], "/")
}

func (ai *DeepAI) TextToImage(model string, params TextToImageParam) (*DeepAIImageGeneratorResponse, error) {
	form := url.Values{}
	form.Add("width", strconv.Itoa(params.Width))
	form.Add("height", strconv.Itoa(params.Height))
	form.Add("text", params.Text)

	if params.GridSize != 0 {
		form.Add("grid_size", strconv.Itoa(params.GridSize))
	}
	if params.NegativeText != "" {
		form.Add("negative_prompt", params.NegativeText)
	}

	selectedServerURL := ai.lb()

	req, err := http.NewRequest("POST", selectedServerURL+"/api/"+model, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("api-key", ai.conf.DeepAIKey)

	resp, err := ai.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", string(body))
	}

	var deepAIImageGeneratorResp DeepAIImageGeneratorResponse
	if err := json.Unmarshal(body, &deepAIImageGeneratorResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	// 当使用自定义服务器代理时，需要将返回的图片地址替换为自定义服务器地址
	if deepAIImageGeneratorResp.OutputURL != "" && !strings.HasPrefix(selectedServerURL, DefaultServerURL) {
		deepAIImageGeneratorResp.OutputURL = strings.ReplaceAll(deepAIImageGeneratorResp.OutputURL, DefaultServerURL, selectedServerURL)
	}

	return &deepAIImageGeneratorResp, nil
}

type TextToImageParam struct {
	Text         string `json:"text"`
	NegativeText string `json:"negative_text"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	GridSize     int    `json:"grid_size"`
}

// DeepAIImageGeneratorResponse 图像生成接口返回结构
type DeepAIImageGeneratorResponse struct {
	ID        string `json:"id"`
	OutputURL string `json:"output_url"`
}

// Upscale 图片放大
func (ai *DeepAI) Upscale(ctx context.Context, imageURL string) (*DeepAIImageGeneratorResponse, error) {
	selectedServerURL := ai.lb()

	resp, err := misc.RestyClient(2).R().
		SetFormData(map[string]string{"image": imageURL}).
		SetHeader("api-key", ai.conf.DeepAIKey).
		SetContext(ctx).
		Post(selectedServerURL + "/api/torch-srgan")
	if err != nil {
		return nil, fmt.Errorf("failed to request: %v", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %s", string(resp.Body()))
	}

	var deepAIImageGeneratorResp DeepAIImageGeneratorResponse
	if err := json.Unmarshal(resp.Body(), &deepAIImageGeneratorResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	// 当使用自定义服务器代理时，需要将返回的图片地址替换为自定义服务器地址
	if deepAIImageGeneratorResp.OutputURL != "" && !strings.HasPrefix(selectedServerURL, DefaultServerURL) {
		deepAIImageGeneratorResp.OutputURL = strings.ReplaceAll(deepAIImageGeneratorResp.OutputURL, DefaultServerURL, selectedServerURL)
	}

	return &deepAIImageGeneratorResp, nil
}

// DrawColor 图片上色
func (ai *DeepAI) DrawColor(ctx context.Context, imageURL string) (*DeepAIImageGeneratorResponse, error) {
	selectedServerURL := ai.lb()
	resp, err := misc.RestyClient(2).R().
		SetFormData(map[string]string{"image": imageURL}).
		SetHeader("api-key", ai.conf.DeepAIKey).
		SetContext(ctx).
		Post(selectedServerURL + "/api/colorizer")
	if err != nil {
		return nil, fmt.Errorf("failed to request: %v", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %s", string(resp.Body()))
	}

	var deepAIImageGeneratorResp DeepAIImageGeneratorResponse
	if err := json.Unmarshal(resp.Body(), &deepAIImageGeneratorResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	// 当使用自定义服务器代理时，需要将返回的图片地址替换为自定义服务器地址
	if deepAIImageGeneratorResp.OutputURL != "" && !strings.HasPrefix(selectedServerURL, DefaultServerURL) {
		deepAIImageGeneratorResp.OutputURL = strings.ReplaceAll(deepAIImageGeneratorResp.OutputURL, DefaultServerURL, selectedServerURL)
	}

	return &deepAIImageGeneratorResp, nil
}
