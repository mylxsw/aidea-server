package dashscope

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	ImageModelSDXL  = "stable-diffusion-xl"
	ImageModelSDV15 = "stable-diffusion-v1.5"
)

type StableDiffusionRequest struct {
	// Model 指明需要调用的模型，可选择stable-diffusion-xl或者stable-diffusion-v1.5
	Model      string                    `json:"model,omitempty"`
	Input      StableDiffusionInput      `json:"input,omitempty"`
	Parameters StableDiffusionParameters `json:"parameters,omitempty"`
}

type StableDiffusionInput struct {
	// Prompt 文本内容， 仅支持英文，不超过75个单词，超过部分会自动截断
	Prompt string `json:"prompt,omitempty"`
	// NegativePrompt 负向文本内容，仅支持英文
	NegativePrompt string `json:"negative_prompt,omitempty"`
}

type StableDiffusionParameters struct {
	// Size 生成图像的分辨率
	//   stable-difussion-v1.5 的 size 固定为 512*512
	//   stable-difussion-xl 的值可支持长宽在 512 和 1024 之间以 128 步长取值的任意组合，如512*1024，1024*768等，默认1024*1024
	Size string `json:"size,omitempty"`
	// N 本次请求生成的图片数量，目前支持1~4张，默认为1
	N int `json:"n,omitempty"`
	// Steps 去噪推理步数，一般步数越大，图像质量越高，步数越小，推理速度越快。 目前默认 50，用户可以在 1-500 间进行调整
	Steps int `json:"steps,omitempty"`
	// Scale 用于指导生成的结果与用户输入的prompt的贴合程度，越高则生成结果与用户输入的prompt更相近。目前默认10，用户可以在1-15之间进行调整
	Scale int `json:"scale,omitempty"`
	// Seed 图片生成时候的种子值，如果不提供，则算法自动用一个随机生成的数字作为种子，如果给定了，
	// 则根据 batch 数量分别生成 seed, seed+1, seed+2, seed+3 为参数的图片。
	Seed int `json:"seed,omitempty"`
}

// StableDiffusion Stable Diffusion 文生图
// 文档：https://help.aliyun.com/zh/dashscope/developer-reference/stable-diffusion-apis
func (ds *DashScope) StableDiffusion(ctx context.Context, req StableDiffusionRequest) (*ImageGenerationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", ds.serviceURL+"/api/v1/services/aigc/text2image/image-synthesis", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+ds.apiKeyLoadBalanced())
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-DashScope-Async", "enable")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	defer httpResp.Body.Close()

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("generate failed [%d]: %s", httpResp.StatusCode, string(data))
	}

	var chatResp ImageGenerationResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	return &chatResp, nil
}
