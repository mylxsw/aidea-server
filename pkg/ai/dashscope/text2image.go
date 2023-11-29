package dashscope

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// 通义万相模型
const (
	// ImageModelText2Image 文本生成图像模型 0.16元/张
	ImageModelText2Image = "wanx-v1"
)

type Text2ImageRequest struct {
	// Model 指明需要调用的模型，固定值 wanx-v1
	Model string `json:"model,omitempty"`
	// Input 本次请求的输入内容
	Input      Text2ImageInput      `json:"input,omitempty"`
	Parameters Text2ImageParameters `json:"parameters,omitempty"`
}

type Text2ImageInput struct {
	// Prompt 文本内容，支持中英文，中文不超过75个字，英文不超过75个单词，超过部分会自动截断
	Prompt string `json:"prompt,omitempty"`
}

type Text2ImageParameters struct {
	// Style 输出图像的风格，目前支持以下风格取值：
	//   "<photography>" 摄影,
	//   "<portrait>" 人像写真,
	//   "<3d cartoon>" 3D卡通,
	//   "<anime>" 动画,
	//   "<oil painting>" 油画,
	//   "<watercolor>"水彩,
	//   "<sketch>" 素描,
	//   "<chinese painting>" 中国画,
	//   "<flat illustration>" 扁平插画,
	//   "<auto>" 默认
	Style string `json:"style,omitempty"`

	// Size 生成图像的分辨率，目前仅支持'1024*1024', '720*1280', '1280*720'三种分辨率，默认为1024*1024像素。
	Size string `json:"size,omitempty"`

	// N 本次请求生成的图片数量，目前支持1~4张，默认为1 (！！！实际测试下来发现默认为 4)。
	N int `json:"n,omitempty"`

	// Seed 图片生成时候的种子值，如果不提供，则算法自动用一个随机生成的数字作为种子，如果给定了，
	// 则根据 batch 数量分别生成 seed, seed+1, seed+2, seed+3 为参数的图片。
	Seed int `json:"seed,omitempty"`
}

const (
	// Text2ImageStylePhotography 摄影
	Text2ImageStylePhotography = "<photography>"
	// Text2ImageStylePortrait 人像写真
	Text2ImageStylePortrait = "<portrait>"
	// Text2ImageStyle3DCartoon 3D卡通
	Text2ImageStyle3DCartoon = "<3d cartoon>"
	// Text2ImageStyleAnime 动画
	Text2ImageStyleAnime = "<anime>"
	// Text2ImageStyleOilPainting 油画
	Text2ImageStyleOilPainting = "<oil painting>"
	// Text2ImageStyleWatercolor 水彩
	Text2ImageStyleWatercolor = "<watercolor>"
	// Text2ImageStyleSketch 素描
	Text2ImageStyleSketch = "<sketch>"
	// Text2ImageStyleChinesePainting 中国画
	Text2ImageStyleChinesePainting = "<chinese painting>"
	// Text2ImageStyleFlatIllustration 扁平插画
	Text2ImageStyleFlatIllustration = "<flat illustration>"
	// Text2ImageStyleAuto 默认
	Text2ImageStyleAuto = "<auto>"
)

// Text2Image 文本生成图像
// 文档：https://help.aliyun.com/zh/dashscope/developer-reference/api-details-9
func (ds *DashScope) Text2Image(ctx context.Context, req Text2ImageRequest) (*ImageGenerationResponse, error) {
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
