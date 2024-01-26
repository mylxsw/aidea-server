package dashscope

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/asteria/log"
	"io"
	"net/http"
)

const (
	// WordArtTextureModel is the name of the WordArt model.
	WordArtTextureModel = "wordart-texture"
	WordArtSemantic     = "wordart-semantic"
)

type WordArtTextureRequest struct {
	Model      string                          `json:"model"`
	Input      WordArtTextureRequestInput      `json:"input"`
	Parameters WordArtTextureRequestParameters `json:"parameters"`
}

type WordArtTextureRequestInput struct {
	// Image 文字图像的地址；
	// 图像要求：黑底白字，图片大小小于5M，图像格式推荐jpg/png/jpeg/bmp，长宽比不大于2，最大边长小于等2048；
	// 若选择了input.image，此字段为必须字段
	Image *WordArtTextureRequestInputImage `json:"image,omitempty"`
	// Text 文字输入的相关字段
	Text *WordArtTextureRequestInputText `json:"text,omitempty"`
	// Prompt 期望文字纹理创意样式的描述提示词，长度小于200，不能为""
	Prompt string `json:"prompt,omitempty"`
	// TextureStyle 纹理风格的类型；
	// 默认为"material"；
	// 取值类型及说明：
	// - "material"：立体材质
	// - "scene": 场景融合
	TextureStyle string `json:"texture_style,omitempty"`
}

const (
	// TextureStyleMaterial 立体材质
	TextureStyleMaterial = "material"
	// TextureStyleScene 场景融合
	TextureStyleScene = "scene"
)

type WordArtTextureRequestInputText struct {
	// TextContent 用户输入的文字内容；
	// 小于6个字；
	// 若选择了input.text，此字段为必须字段，且不能为空字符串""；
	// 支持中文、阿拉伯数字、英文字母（字符的支持范围由ttf文件决定）
	TextContent string `json:"text_content,omitempty"`
	// TtfURL 用户传入的ttf文件；
	// 标准的ttf文件，文件大小小于30M；
	// 当使用input.text时，input.text.ttf_url 和 input.text.font_name 需要二选一
	TtfURL string `json:"ttf_url,omitempty"`
	// FontName 使用预置字体的名称；
	//
	// 当使用input.text时，input.text.ttf_url 和 input.text.font_name 需要二选一；
	//
	// 默认为"dongfangdakai"
	//
	// 取值类型及说明：
	// - 'dongfangdakai'：阿里妈妈东方大楷
	// - 'puhuiti_m'：阿里巴巴普惠体
	// - 'shuheiti'：阿里妈妈数黑体
	// - 'jinbuti'：钉钉进步体
	// - 'kuheiti'：站酷酷黑体
	// - 'kuaileti'：站酷快乐体
	// - 'wenyiti'：站酷文艺体
	// - 'logoti'：站酷小薇LOGO体
	// - 'cangeryuyangti_m'：站酷仓耳渔阳体
	// - 'siyuansongti_b'：思源宋体
	// - 'siyuanheiti_m'：思源黑体
	// - 'fangzhengkaiti'：方正楷体
	FontName string `json:"font_name,omitempty"`
	// 文字输入的图片的宽高比；
	// 默认为"1:1"，可选的比例有："1:1", "16:9", "9:16"；
	OutputImageRatio string `json:"output_image_ratio,omitempty"`
}

type WordArtTextureRequestInputImage struct {
	ImageURL string `json:"image_url,omitempty"`
}

type WordArtTextureRequestParameters struct {
	// ImageShortSize 生成的图片短边的长度，默认为704，取值范围为[512, 1024]，
	// 若输入数值非64的倍数，则最终取值为不大于该数值的能被64整除的最大数
	ImageShortSize int `json:"image_short_size,omitempty"`
	// N 生成的图片数量，默认为 1，取值范围为[1, 4]
	N int `json:"n,omitempty"`
	// AlphaChannel 是否返回带alpha通道的图片；默认为 false；
	AlphaChannel bool `json:"alpha_channel,omitempty"`
}

func (ds *DashScope) WordArtTexture(ctx context.Context, req WordArtTextureRequest) (*ImageGenerationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	log.With(req).Debugf("send wordart texture request")

	httpReq, err := http.NewRequestWithContext(ctx, "POST", ds.serviceURL+"/api/v1/services/aigc/wordart/texture", bytes.NewReader(body))
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
