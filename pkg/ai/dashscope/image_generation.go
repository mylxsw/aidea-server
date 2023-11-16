package dashscope

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ImageGenerationRequest struct {
	// Model 指明需要调用的模型，固定值 wanx-style-repaint-v1
	Model string `json:"model,omitempty"`
	// Input 本次请求的输入内容
	Input ImageGenerationRequestInput `json:"input,omitempty"`
}

type ImageGenerationRequestInput struct {
	// ImageURL 输入的图像 URL，
	// 分辨率：可支持输入分辨率范围：不小于256*256，不超过5760*3240, 长宽比不超过1.78 : 1；为保证生成质量，最佳分辨率输入范围：不小于512*512，不超过2048*1024
	// 为确保生成质量，请上传脸部清晰照片，人脸比例不宜过小，并避免夸张姿势和表情
	// 类型：JPEG，PNG，JPG，BMP，WEBP
	// 大小：不超过10M
	ImageURL string `json:"image_url,omitempty"`
	// StyleIndex 想要生成的风格化类型索引：
	// 0 复古漫画
	// 1 3D童话
	// 2 二次元
	// 3 小清新
	// 4 未来科技
	// 5 3D写实
	StyleIndex int `json:"style_index"`
}

const (
	// ImageStyleComic 复古漫画
	ImageStyleComic = 0
	// ImageStyle3DFairyTale 3D童话
	ImageStyle3DFairyTale = 1
	// ImageStyleCospa 二次元
	ImageStyleCospa = 2
	// ImageStyleFresh 小清新
	ImageStyleFresh = 3
	// ImageStyleFuture 未来科技
	ImageStyleFuture = 4
	// ImageStyle3DReal 3D写实
	ImageStyle3DReal = 5
)

type ImageGenerationResponse struct {
	// RequestID 本次请求的系统唯一码
	RequestID string `json:"request_id,omitempty"`
	// Output 本次请求的输出内容
	Output ImageGenerationOutput `json:"output,omitempty"`
}

type ImageGenerationOutput struct {
	// TaskID 本次请求的异步任务的作业 id，实际作业结果需要通过异步任务查询接口获取。
	TaskID string `json:"task_id,omitempty"`
	// TaskStatus 提交异步任务后的作业状态。
	TaskStatus string `json:"task_status,omitempty"`
}

// 通义万相模型
const (
	// ImageModelPersonRepaint 人像风格重绘模型 0.12元/张
	ImageModelPersonRepaint = "wanx-style-repaint-v1"
)

// ImageGeneration 人像风格重绘
// 文档：https://help.aliyun.com/zh/dashscope/developer-reference/tongyi-wanxiang-style-repaint
func (ds *DashScope) ImageGeneration(ctx context.Context, req ImageGenerationRequest) (*ImageGenerationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", ds.serviceURL+"/api/v1/services/aigc/image-generation/generation", bytes.NewReader(body))
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
