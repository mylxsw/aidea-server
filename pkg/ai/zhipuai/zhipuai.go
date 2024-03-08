package zhipuai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/token"
	"io"
	"net/http"
	"strings"
	"time"
)

// ModelGLM4V 效果不好，暂不启用
const ModelGLM4V = "glm-4v"

const ModelGLM4 = "glm-4"
const ModelGLM3Turbo = "glm-3-turbo"

type ZhipuAI struct {
	apiKey string
}

func NewZhipuAI(apiKey string) *ZhipuAI {
	return &ZhipuAI{apiKey: apiKey}
}

// createAuthToken 生成鉴权 Token
func (ai *ZhipuAI) createAuthToken() string {
	segs := strings.SplitN(ai.apiKey, ".", 2)
	id, secret := segs[0], segs[1]

	return token.New(secret).CreateCustomToken(
		map[string]any{
			"sign_type": "SIGN",
			"alg":       "HS256",
		},
		token.Claims{
			"api_key":   id,
			"exp":       time.Now().Add(time.Hour).UnixMilli(),
			"timestamp": time.Now().UnixMilli(),
		},
	)
}

type ChatRequest struct {
	// Model 所要调用的模型编码
	Model string `json:"model"`
	// Messages 调用语言模型时，将当前对话信息列表作为提示输入给模型
	Messages []any `json:"messages"`

	// Stream 使用同步调用时，此参数应当设置为 false 或者省略。表示模型生成完所有内容后一次性返回所有内容。
	// 如果设置为 true，模型将通过标准 Event Stream ，逐块返回模型生成内容。Event Stream 结束时会返回一条data: [DONE]消息。
	Stream bool `json:"stream,omitempty"`

	// RequestID 由用户端传参，需保证唯一性；用于区分每次请求的唯一标识，用户端不传时平台会默认生成。
	RequestID string `json:"request_id,omitempty"`
	// DoSample 为 true 时启用采样策略，do_sample 为 false 时采样策略 temperature、top_p 将不生效
	DoSample bool `json:"do_sample,omitempty"`
	// Temperature 采样温度，控制输出的随机性，必须为正数
	// 取值范围是：(0.0, 1.0)，不能等于 0，默认值为 0.95，值越大，会使输出更随机，更具创造性；值越小，输出会更加稳定或确定
	// 建议您根据应用场景调整 top_p 或 temperature 参数，但不要同时调整两个参数
	Temperature float64 `json:"temperature,omitempty"`
	// TopP 用温度取样的另一种方法，称为核取样
	// 取值范围是：(0.0, 1.0) 开区间，不能等于 0 或 1，默认值为 0.7
	// 模型考虑具有 top_p 概率质量 tokens 的结果
	// 例如：0.1 意味着模型解码器只考虑从前 10% 的概率的候选集中取 tokens
	// 建议您根据应用场景调整 top_p 或 temperature 参数，但不要同时调整两个参数
	TopP float64 `json:"top_p,omitempty"`
	// MaxToken 模型输出最大 tokens
	MaxToken int `json:"max_token,omitempty"`
	// Stop 模型在遇到 stop 所制定的字符时将停止生成，目前仅支持单个停止词，格式为["stop_word1"]
	Stop []string `json:"stop,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type MultipartMessage struct {
	Role    string             `json:"role"`
	Content []MultipartContent `json:"content"`
}

type MultipartContent struct {
	// Type 类型
	// 用户输入：text
	// 图片类型：image_url
	Type string `json:"type"`
	// Text type 是 text 时补充
	Text string `json:"text,omitempty"`
	// ImageURL type 是 image_url 时补充
	ImageURL *MultipartContentImage `json:"image_url,omitempty"`
}

type MultipartContentImage struct {
	URL string `json:"url"`
}

type ChatResponse struct {
	// ID 任务 ID
	ID string `json:"id,omitempty"`
	// Created 请求创建时间，是以秒为单位的 Unix 时间戳
	Created int64 `json:"created,omitempty"`
	// Model 模型名称
	Model string `json:"model,omitempty"`
	// Choices 当前对话的模型输出内容
	Choices []Choice `json:"choices,omitempty"`
	// Usage 结束时返回本次模型调用的 tokens 数量统计
	Usage Usage `json:"usage,omitempty"`

	// Error 错误信息
	Error *Error `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type Choice struct {
	// Index 结果下标
	Index int64 `json:"index,omitempty"`
	// FinishReason 模型推理终止的原因
	// stop 代表推理自然结束或触发停止词
	// tool_calls 代表模型命中函数
	// length 代表到达 tokens 长度上限
	FinishReason string `json:"finish_reason,omitempty"`
	// Message 模型返回的文本信息
	Message Message `json:"message,omitempty"`

	// Delta 模型返回的文本信息的增量(stream 模式)
	Delta Message `json:"delta,omitempty"`
}

type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens,omitempty"`
}

func (ai *ZhipuAI) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	resp, err := misc.RestyClient(2).R().
		SetContext(ctx).
		SetBody(req).
		SetHeader("Authorization", ai.createAuthToken()).
		SetHeader("Content-Type", "application/json").
		Post("https://open.bigmodel.cn/api/paas/v4/chat/completions")
	if err != nil {
		return nil, err
	}

	respData := resp.Body()

	var chatResponse ChatResponse
	if err := json.Unmarshal(respData, &chatResponse); err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("chat failed, status code: %d, %s", resp.StatusCode(), chatResponse.Error.Message)
	}

	if chatResponse.Error != nil && chatResponse.Error.Code == "200" {
		chatResponse.Error.Code = ""
	}

	return &chatResponse, nil
}

func (ai *ZhipuAI) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://open.bigmodel.cn/api/paas/v4/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", ai.createAuthToken())
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()

		return nil, fmt.Errorf("chat failed [%s]: %s", httpResp.Status, string(data))
	}

	res := make(chan ChatResponse)
	go func() {
		defer func() {
			_ = httpResp.Body.Close()
			close(res)
		}()

		reader := bufio.NewReader(httpResp.Body)
		for {
			data, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return
				}

				select {
				case <-ctx.Done():
				case res <- ChatResponse{Error: &Error{Code: "100", Message: fmt.Sprintf("read response failed: %v", err)}}:
				}
				return
			}

			dataStr := strings.TrimSpace(string(data))
			if dataStr == "" {
				continue
			}

			if !strings.HasPrefix(dataStr, "data:") {
				continue
			}

			var chatResponse ChatResponse
			if err := json.Unmarshal([]byte(dataStr[5:]), &chatResponse); err != nil {
				select {
				case <-ctx.Done():
				case res <- ChatResponse{Error: &Error{Code: "100", Message: fmt.Sprintf("decode response failed: %v", err)}}:
				}
				return
			}

			if chatResponse.Error != nil && chatResponse.Error.Code == "200" {
				chatResponse.Error.Code = ""
			}

			select {
			case <-ctx.Done():
				return
			case res <- chatResponse:
				if chatResponse.Choices[0].FinishReason != "" {
					return
				}
			}
		}
	}()

	return res, nil
}
