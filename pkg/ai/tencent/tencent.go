package tencent

import (
	"fmt"
	aiart "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/aiart/v20221229"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

type TencentArt struct {
	client *aiart.Client
}

func NewTencentArt(secretID, secretKey string) (*TencentArt, error) {
	credential := common.NewCredential(secretID, secretKey)

	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "aiart.tencentcloudapi.com"

	client, err := aiart.NewClient(credential, "ap-shanghai", cpf)
	if err != nil {
		return nil, err
	}

	return &TencentArt{client: client}, nil
}

type ImageToImageRequest struct {
	Prompt         string `json:"prompt,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	// Style 参考值 https://cloud.tencent.com/document/product/1668/86250
	// 106 - 水彩画
	// 110 - 2.5D
	// 201 - 日系动漫
	// 202 - 美系动漫
	// 203 - 唯美古风
	Style       string  `json:"style,omitempty"`
	Strength    float64 `json:"strength,omitempty"`
	ImageBase64 string  `json:"image_base64,omitempty"`
	ImageURL    string  `json:"image_url,omitempty"`
}

func (art *TencentArt) ImageToImage(req ImageToImageRequest) (string, error) {
	genReq := aiart.NewImageToImageRequest()
	genReq.Prompt = common.StringPtr(req.Prompt)
	genReq.NegativePrompt = common.StringPtr(req.NegativePrompt)

	if req.ImageURL != "" {
		genReq.InputUrl = common.StringPtr(req.ImageURL)
	}

	if req.ImageBase64 != "" {
		genReq.InputImage = common.StringPtr(req.ImageBase64)
	}

	if req.Style != "" {
		genReq.Styles = []*string{common.StringPtr(req.Style)}
	}

	if req.Strength > 0 {
		genReq.Strength = common.Float64Ptr(req.Strength)
	}

	// 支持 base64、url
	genReq.RspImgType = common.StringPtr("base64")
	resp, err := art.client.ImageToImage(genReq)
	if err != nil {
		if _, ok := err.(*errors.TencentCloudSDKError); ok {
			return "", fmt.Errorf("An API error has returned: %s", err)
		}

		return "", err
	}

	return *resp.Response.ResultImage, nil
}
