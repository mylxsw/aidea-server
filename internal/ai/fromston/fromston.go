package fromston

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/uploader"
	"gopkg.in/resty.v1"
)

type Fromston struct {
	conf  *config.Config
	resty *resty.Client
}

func NewFromston(conf *config.Config) *Fromston {
	client := helper.RestyClient(2).SetTimeout(180 * time.Second)
	return &Fromston{conf: conf, resty: client}
}

type GenImageRequest struct {
	// Prompt <=500 画面描述，填写画面中的内容描述、艺术家风格等
	Prompt string `json:"prompt,omitempty"`
	// FillPrompt 是否开始禅思模式：0=否，1=是；开启禅思模式时，将根据上传的prompt自动优化文本描述，以实现更佳效果
	FillPrompt int64 `json:"fill_prompt,omitempty"`
	// Width 生成图片的宽度
	Width int64 `json:"width,omitempty"`
	// Height 生成图片的高度
	Height int64 `json:"height,omitempty"`
	// RefImg 参考图链接（需上传至6pen图床）
	RefImg string `json:"ref_img,omitempty"`
	// 模型类型：枚举值: preset，third
	ModelType string `json:"model_type,omitempty"`
	// 模型ID
	ModelID int64 `json:"model_id,omitempty"`
	// Multiply 生成图片数量
	Multiply int64             `json:"multiply,omitempty"`
	Addition *GenImageAddition `json:"addition,omitempty"`
}

type GenImageAddition struct {
	// Strength strength重绘幅度说明：
	// 当使用了参考图时，这个参数越小，结果越像你的参考图本身
	// 当未使用参考图时，这个参数影响高清修复的程度，即参数越大，结果越清晰，但很可能出现奇怪的东西，参数越小，结果相较而言会模糊，但有时也能呈现更好的艺术效果
	// 根据经验，0.3~0.7是不错的选择
	Strength float64 `json:"strength,omitempty"`
	// CfgScale 描述权重，描述权重决定了你即将生成的作品，在多大程度上和你描述相符。权重越高，模型越听你的话，但同时模型自由发挥的空间越小。一般将权重设置为 7-15 之间是合理的选择
	CfgScale int64 `json:"cfg_scale,omitempty"`
	// NegativePrompt 反向词，如果你不希望图片出现什么元素，就写在反向词里。反向词仅支持输入英文单词，请勿输入中文或长句
	NegativePrompt string `json:"negative_prompt,omitempty"`
	// ImgFmt 图片生成格式，png的质量会更高，三方模型不支持 png jpg
	ImgFmt string `json:"img_fmt,omitempty"`
}

type Response[T any] struct {
	Code    int64  `json:"code"`
	Info    string `json:"info"`
	Data    T      `json:"data,omitempty"`
	Results T      `json:"results,omitempty"`
}

type GenImageResponseData struct {
	ID        string   `json:"id,omitempty"`
	Estimate  int64    `json:"estimate,omitempty"`
	IDs       []string `json:"ids,omitempty"`
	Estimates []int64  `json:"estimates,omitempty"`
}

func (art *Fromston) GenImage(ctx context.Context, req GenImageRequest) (*GenImageResponseData, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := art.resty.R().
		SetBody(reqBody).
		SetHeader("ys-api-key", art.conf.FromstonKey).
		SetHeader("Content-Type", "application/json").
		Post(art.conf.FromstonServer + "/release/open-task")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("request failed: [%d %s] %s", resp.StatusCode(), resp.Status(), resp.String())
	}

	var res Response[GenImageResponseData]
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		return nil, err
	}

	if res.Code != 200 {
		return nil, fmt.Errorf("request failed:[%d] %s", res.Code, res.Info)
	}

	return &res.Data, nil
}

type Task struct {
	ID         string `json:"id,omitempty"`
	RefImg     string `json:"ref_img,omitempty"`
	Height     int64  `json:"height,omitempty"`
	Width      int64  `json:"width,omitempty"`
	FillPrompt int64  `json:"fill_prompt,omitempty"`
	ModelID    int64  `json:"model_id,omitempty"`
	ModelType  string `json:"model_type,omitempty"`
	Estimate   int64  `json:"estimate,omitempty"`
	Seed       int64  `json:"seed,omitempty"`
	GenImg     string `json:"gen_img,omitempty"`
	FailReson  string `json:"fail_reason,omitempty"`
	State      string `json:"state,omitempty"`
	StateText  string `json:"state_text,omitempty"`
	Cost       int64  `json:"cost,omitempty"`
}

func (task Task) UploadResources(ctx context.Context, up *uploader.Uploader, uid int64) (string, error) {
	ret, err := up.UploadRemoteFile(ctx, task.GenImg, int(uid), uploader.DefaultUploadExpireAfterDays, "png", false)
	if err != nil {
		return "", fmt.Errorf("upload image to qiniu failed: %w", err)
	}

	return ret, nil
}

func (art *Fromston) QueryTask(ctx context.Context, id string) (*Task, error) {
	resp, err := art.resty.R().
		SetHeader("ys-api-key", art.conf.FromstonKey).
		SetQueryParam("id", id).
		Get(art.conf.FromstonServer + "/release/open-task")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("request failed: [%d %s] %s", resp.StatusCode(), resp.Status(), resp.String())
	}

	var res Response[Task]
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		return nil, err
	}

	if res.Code != 200 {
		return nil, fmt.Errorf("request failed:[%d] %s", res.Code, res.Info)
	}

	return &res.Data, nil
}

func (art *Fromston) QueryTasks(ctx context.Context, ids []string) ([]Task, error) {
	resp, err := art.resty.R().
		SetHeader("ys-api-key", art.conf.FromstonKey).
		SetQueryParam("ids", strings.Join(ids, ",")).
		Get(art.conf.FromstonServer + "/release/open-task")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("request failed: [%d %s] %s", resp.StatusCode(), resp.Status(), resp.String())
	}

	var res Response[[]Task]
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		return nil, err
	}

	if res.Code != 200 {
		return nil, fmt.Errorf("request failed:[%d] %s", res.Code, res.Info)
	}

	return res.Results, nil
}

type Model struct {
	Type    string `json:"type,omitempty"`
	ModelID int64  `json:"model_id,omitempty"`
	Name    string `json:"name,omitempty"`
	// ArtistStyle 标记词，如果不为空需要添加到 prompt，否则不会生效
	ArtistStyle string `json:"artist_style,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"` // 自定义字段，非接口返回
	IntroURL    string `json:"intro_url,omitempty"`
}

func (art *Fromston) Models(ctx context.Context) ([]Model, error) {
	resp, err := art.resty.R().
		SetHeader("ys-api-key", art.conf.FromstonKey).
		SetQueryParam("page_size", "100").
		Get(art.conf.FromstonServer + "/release/open-task/models")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("request failed: [%d %s] %s", resp.StatusCode(), resp.Status(), resp.String())
	}

	var res Response[[]Model]
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		return nil, err
	}

	if res.Code != 200 {
		return nil, fmt.Errorf("request failed:[%d] %s", res.Code, res.Info)
	}

	return res.Results, nil
}

type UploadData struct {
	Hash string `json:"hash,omitempty"`
	Key  string `json:"key,omitempty"`
}

func (art *Fromston) UploadImage(ctx context.Context, file string) (string, error) {
	resp, err := art.resty.R().
		SetHeader("ys-api-key", art.conf.FromstonKey).
		SetFile("ref_img", file).
		Post(art.conf.FromstonServer + "/release/upload")
	if err != nil {
		return "", err
	}

	if !resp.IsSuccess() {
		return "", fmt.Errorf("request failed: [%d %s] %s", resp.StatusCode(), resp.Status(), resp.String())
	}

	var res Response[UploadData]
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		return "", err
	}

	if res.Code != 200 {
		return "", fmt.Errorf("request failed:[%d] %s", res.Code, res.Info)
	}

	return "https://sourceimg.6pen.art/" + res.Data.Key, nil
}
