package baidu

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/asteria/log"
	"gopkg.in/resty.v1"
	"io"
	"net/http"
	"sync"
)

type BaiduImageAI struct {
	APIKey      string
	APISecret   string
	accessToken string
	lock        sync.RWMutex
}

func NewBaiduImageAI(apiKey, apiSecret string) *BaiduImageAI {
	ai := &BaiduImageAI{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}

	if err := ai.RefreshAccessToken(); err != nil {
		log.Errorf("refresh baidu ai access token failed: %s", err)
	}

	return ai
}

// RefreshAccessToken 刷新 AccessToken
func (ai *BaiduImageAI) RefreshAccessToken() error {
	resp, err := resty.R().
		SetQueryParam("grant_type", "client_credentials").
		SetQueryParam("client_id", ai.APIKey).
		SetQueryParam("client_secret", ai.APISecret).
		Post("https://aip.baidubce.com/oauth/2.0/token")
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("refresh access token failed, status code: %d", resp.StatusCode())
	}

	var accessTokenResponse RefreshAccessTokenResponse
	if err := json.Unmarshal(resp.Body(), &accessTokenResponse); err != nil {
		return err
	}

	if accessTokenResponse.AccessToken != "" {
		ai.lock.Lock()
		ai.accessToken = accessTokenResponse.AccessToken
		ai.lock.Unlock()
	}

	return nil
}

func (ai *BaiduImageAI) getAccessToken() string {
	ai.lock.RLock()
	defer ai.lock.RUnlock()
	return ai.accessToken
}

type ImageStyleTransRequest struct {
	// Image ase64编码后大小不超过10M (参考：原图大约为8M以内），最短边至少10px，最长边最大5000px
	// 长宽比4：1以内。注意：图片的base64编码是不包含图片头的，如（data:image/jpg;base64,）
	Image string `json:"image,omitempty"`
	// URL 图片完整URL，URL长度不超过1024字节，URL对应的图片base64编码后大小不超过10M(参考：原图大约为8M以内），
	// 最短边至少10px，最长边最大5000px，长宽比4：1以内,支持jpg/png/bmp格式，当image字段存在时url字段失效。
	URL string `json:"url,omitempty"`
	// Option 选择风格
	//   - cartoon：卡通画风格
	//   - pencil：铅笔风格
	//   - color_pencil：彩色铅笔画风格
	//   - warm：彩色糖块油画风格
	//   - wave：神奈川冲浪里油画风格
	//   - lavender：薰衣草油画风格
	//   - mononoke：奇异油画风格
	//   - scream：呐喊油画风格
	//   - gothic：哥特油画风格
	Option string `json:"option"`
}

func (req ImageStyleTransRequest) ToFormData() map[string]string {
	data := map[string]string{}

	if req.Image != "" {
		data["image"] = req.Image
	}

	if req.URL != "" {
		data["url"] = req.URL
	}

	data["option"] = req.Option
	return data
}

type ImageResponse struct {
	LogID     int64  `json:"log_id,omitempty"`
	Image     string `json:"image,omitempty"`
	ErrorCode int64  `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

func (resp ImageResponse) WriteTo(target io.Writer) (int64, error) {
	data, err := base64.StdEncoding.DecodeString(resp.Image)
	if err != nil {
		return 0, fmt.Errorf("failed to decode image: %w", err)
	}

	n, err := target.Write(data)
	if err != nil {
		return 0, fmt.Errorf("failed to write image: %w", err)
	}

	return int64(n), nil
}

// ImageStyleTrans 图像风格转换
func (ai *BaiduImageAI) ImageStyleTrans(ctx context.Context, req ImageStyleTransRequest) (*ImageResponse, error) {
	resp, err := helper.RestyClient(2).R().
		SetFormData(req.ToFormData()).
		SetQueryParam("access_token", ai.getAccessToken()).
		SetContext(ctx).
		Post("https://aip.baidubce.com/rest/2.0/image-process/v1/style_trans")
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %s", string(resp.Body()))
	}

	var ret ImageResponse
	if err := json.Unmarshal(resp.Body(), &ret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	if ret.ErrorCode > 0 {
		return nil, fmt.Errorf("request failed: [%d] %s", ret.ErrorCode, ret.ErrorMsg)
	}

	return &ret, nil
}

type SelfieAnimeRequest struct {
	// Image ase64编码后大小不超过10M (参考：原图大约为8M以内），最短边至少10px，最长边最大5000px
	// 长宽比4：1以内。注意：图片的base64编码是不包含图片头的，如（data:image/jpg;base64,）
	Image string `json:"image,omitempty"`
	// URL 图片完整URL，URL长度不超过1024字节，URL对应的图片base64编码后大小不超过10M(参考：原图大约为8M以内），
	// 最短边至少10px，最长边最大5000px，长宽比4：1以内,支持jpg/png/bmp格式，当image字段存在时url字段失效。
	URL string `json:"url,omitempty"`

	// Type anime或者anime_mask。前者生成二次元动漫图，后者生成戴口罩的二次元动漫人像
	Type string `json:"type,omitempty"`
	// MaskID 在type参数填入anime_mask时生效，1～8之间的整数，用于指定所使用的口罩的编码。
	// type参数没有填入anime_mask，或mask_id 为空时，生成不戴口罩的二次元动漫图。
	MaskID int `json:"mask_id,omitempty"`
}

func (req SelfieAnimeRequest) ToFormData() map[string]string {
	data := map[string]string{}

	if req.Image != "" {
		data["image"] = req.Image
	}

	if req.URL != "" {
		data["url"] = req.URL
	}

	if req.Type != "" {
		data["type"] = req.Type
	}

	if req.MaskID > 0 {
		data["mask_id"] = fmt.Sprintf("%d", req.MaskID)
	}
	return data
}

// SelfieAnime 人像动漫化
func (ai *BaiduImageAI) SelfieAnime(ctx context.Context, req SelfieAnimeRequest) (*ImageResponse, error) {
	resp, err := helper.RestyClient(2).R().
		SetFormData(req.ToFormData()).
		SetQueryParam("access_token", ai.getAccessToken()).
		SetContext(ctx).
		Post("https://aip.baidubce.com/rest/2.0/image-process/v1/selfie_anime")
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %s", string(resp.Body()))
	}

	var ret ImageResponse
	if err := json.Unmarshal(resp.Body(), &ret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	if ret.ErrorCode > 0 {
		return nil, fmt.Errorf("request failed: [%d] %s", ret.ErrorCode, ret.ErrorMsg)
	}

	return &ret, nil
}

type SimpleImageRequest struct {
	// Image ase64编码后大小不超过10M (参考：原图大约为8M以内），最短边至少10px，最长边最大5000px
	// 长宽比4：1以内。注意：图片的base64编码是不包含图片头的，如（data:image/jpg;base64,）
	Image string `json:"image,omitempty"`
	// URL 图片完整URL，URL长度不超过1024字节，URL对应的图片base64编码后大小不超过10M(参考：原图大约为8M以内），
	// 最短边至少10px，最长边最大5000px，长宽比4：1以内,支持jpg/png/bmp格式，当image字段存在时url字段失效。
	URL string `json:"url,omitempty"`
}

func (req SimpleImageRequest) ToFormData() map[string]string {
	data := map[string]string{}

	if req.Image != "" {
		data["image"] = req.Image
	}

	if req.URL != "" {
		data["url"] = req.URL
	}

	return data
}

// Colourize 照片上色
func (ai *BaiduImageAI) Colourize(ctx context.Context, req SimpleImageRequest) (*ImageResponse, error) {
	resp, err := helper.RestyClient(2).R().
		SetFormData(req.ToFormData()).
		SetQueryParam("access_token", ai.getAccessToken()).
		SetContext(ctx).
		Post("https://aip.baidubce.com/rest/2.0/image-process/v1/colourize")
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %s", string(resp.Body()))
	}

	var ret ImageResponse
	if err := json.Unmarshal(resp.Body(), &ret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	if ret.ErrorCode > 0 {
		return nil, fmt.Errorf("request failed: [%d] %s", ret.ErrorCode, ret.ErrorMsg)
	}

	return &ret, nil
}

// QualityEnhance 图像无损放大
func (ai *BaiduImageAI) QualityEnhance(ctx context.Context, req SimpleImageRequest) (*ImageResponse, error) {
	resp, err := helper.RestyClient(2).R().
		SetFormData(req.ToFormData()).
		SetQueryParam("access_token", ai.getAccessToken()).
		SetContext(ctx).
		Post("https://aip.baidubce.com/rest/2.0/image-process/v1/image_quality_enhance")
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %s", string(resp.Body()))
	}

	var ret ImageResponse
	if err := json.Unmarshal(resp.Body(), &ret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	if ret.ErrorCode > 0 {
		return nil, fmt.Errorf("request failed: [%d] %s", ret.ErrorCode, ret.ErrorMsg)
	}

	return &ret, nil
}
