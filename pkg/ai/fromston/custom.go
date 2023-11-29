package fromston

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type GenImageCustomRequest struct {
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
	// 模型ID
	ModelID string `json:"model_id,omitempty"`
	// Multiply 生成图片数量
	Multiply int64             `json:"multiply,omitempty"`
	Addition *GenImageAddition `json:"addition,omitempty"`
}

func (art *Fromston) GenImageCustom(ctx context.Context, req GenImageCustomRequest) (*GenImageResponseData, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := art.resty.R().
		SetBody(reqBody).
		SetHeader("ys-api-key", art.conf.FromstonKey).
		SetHeader("Content-Type", "application/json").
		Post(art.conf.FromstonServer + "/release/open-custom-model/task")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		if resp.StatusCode() == 422 {
			return nil, errors.New("检测到违规内容，请修改后重试")
		}

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

func (art *Fromston) QueryCustomTask(ctx context.Context, id string) (*Task, error) {
	resp, err := art.resty.R().
		SetHeader("ys-api-key", art.conf.FromstonKey).
		SetQueryParam("id", id).
		Get(art.conf.FromstonServer + "/release/open-custom-model/task")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		if resp.StatusCode() == 422 {
			return nil, errors.New("检测到违规内容，请修改后重试")
		}

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
