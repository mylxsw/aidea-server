package dashscope

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
)

type FaceChainPersonDetectRequest struct {
	// Model 指明需要调用的模型，固定值 facechain-facedetect
	Model string                     `json:"model,omitempty"`
	Input FaceChainPersonDetectInput `json:"input,omitempty"`
}

type FaceChainPersonDetectInput struct {
	// Images 输入的图像 URL，分辨率不小于256*256，不超过4096*4096，文件大小不超过 5MB
	// 支持格式包括JPEG, PNG, JPG, WEBP
	Images []string `json:"images,omitempty"`
}

type FaceChainPersonDetectResponse struct {
	// RequestID 本次请求的系统唯一码
	RequestID string                      `json:"request_id,omitempty"`
	Output    FaceChainPersonDetectOutput `json:"output,omitempty"`
}

type FaceChainPersonDetectOutput struct {
	// IsFace 客户提交的图像列表对应的检查结果
	IsFace []bool `json:"is_face,omitempty"`
}

// FaceChainDetect 人物图像检测 API
// https://help.aliyun.com/zh/dashscope/developer-reference/facechain-face-detection?spm=a2c4g.11186623.0.0.659466b5S9Xqng
func (ds *DashScope) FaceChainDetect(ctx context.Context, req FaceChainPersonDetectInput) (*FaceChainPersonDetectResponse, error) {
	resp, err := misc.RestyClient(2).R().
		SetHeader("Authorization", "Bearer "+ds.apiKeyLoadBalanced()).
		SetHeader("Content-Type", "application/json").
		SetContext(ctx).
		SetBody(FaceChainPersonDetectRequest{Input: req, Model: "facechain-facedetect"}).
		Post(ds.serviceURL + "/api/v1/services/vision/facedetection/detect")
	if err != nil {
		return nil, fmt.Errorf("failed to request: %v", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %s", string(resp.Body()))
	}

	var ret FaceChainPersonDetectResponse
	if err := json.Unmarshal(resp.Body(), &ret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	return &ret, nil
}
