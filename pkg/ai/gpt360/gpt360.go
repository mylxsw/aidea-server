package gpt360

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// Model360GPT_S2_V9 ¥0.012 / 1K tokens
	Model360GPT_S2_V9 = "360GPT_S2_V9"
	// Model360GPT_S2_V8 ¥0.1 / images
	Model360CV_S0_V5 = "360CV_S0_V5"
	// 360CV_StyleTransfer_V1 ¥0.1 / images
	Model360CV_StyleTransfer_V1 = "360CV_StyleTransfer_V1"
)

type GPT360 struct {
	apiKey string
}

func NewGPT360(apiKey string) *GPT360 {
	return &GPT360{apiKey: apiKey}
}

type ChatRequest struct {
	Model    string   `json:"model"`
	Messages Messages `json:"messages"`
	Stream   bool     `json:"stream"`
	Temperature float64  `json:"temperature,omitempty"`
	// MaxTokens 大于等于1小于等于2048，默认值是2048，代表输出结果的最大token数
	MaxTokens int `json:"max_tokens,omitempty"`
	// TopP 大于等于0小于等于1，默认值是 0.5
	TopP float64 `json:"top_p,omitempty"`
	TokK int     `json:"tok_k,omitempty"`
	// RepetitionPenalty 取值应大于等于1小于等于2，默认值是1.05
	RepetitionPenalty float64 `json:"repetition_penalty,omitempty"`
	// NumBeams 取值应大于等于1小于等于5，默认值是1
	NumBeams int `json:"num_beams,omitempty"`
	// User 标记业务方用户id，便于业务方区分不同用户
	User string `json:"user,omitempty"`
}

type Messages []Message

type Message struct {
	// Role 取值有 system, assistant, user
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	ID      string        `json:"id,omitempty"`
	Model   string        `json:"model,omitempty"`
	Created int64         `json:"created,omitempty"`
	Choices []Choice      `json:"choices,omitempty"`
	Usage   ChatUsage     `json:"usage,omitempty"`
	Error   ErrorResponse `json:"error,omitempty"`
}

type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type Choice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message,omitempty"`
	// Delta for stream mode
	Delta struct {
		Content string `json:"content"`
	} `json:"delta,omitempty"`
	// FinishReason 流式返回时，一般是空字符串；当命中敏感词时，最后一条该字段值是content_filter
	FinishReason string `json:"finish_reason,omitempty"`
}

// Chat 发起对话
func (g360 *GPT360) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.360.cn/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+g360.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	defer httpResp.Body.Close()

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		errResponse := tryParseErrorResponse(httpResp.Body)
		if err := errResponse.Error(); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("gpt360 chat failed, status [%d], code [%s]: %s", httpResp.StatusCode, errResponse.Code, errResponse.Message)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	return &chatResp, nil
}

func (g360 *GPT360) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.360.cn/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request failed: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+g360.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		errResponse := tryParseErrorResponse(httpResp.Body)
		_ = httpResp.Body.Close()
		if err := errResponse.Error(); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("gpt360 chat failed, status [%d], code [%s]: %s", httpResp.StatusCode, errResponse.Code, errResponse.Message)
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
				case res <- ChatResponse{
					Error: ErrorResponse{
						Code:    "100",
						Message: fmt.Sprintf("read stream failed: %s", err.Error()),
					},
				}:
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

			if dataStr[6:] == "[DONE]" {
				return
			}

			var chatResponse ChatResponse
			if err := json.Unmarshal([]byte(dataStr[6:]), &chatResponse); err != nil {
				select {
				case <-ctx.Done():
				case res <- ChatResponse{
					Error: ErrorResponse{
						Code:    "101",
						Message: fmt.Sprintf("unmarshal stream data failed: %v", err),
					},
				}:
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case res <- chatResponse:
			}
		}
	}()

	return res, nil
}

type ErrorResponse struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e ErrorResponse) Error() error {
	if e.Code == "" {
		return nil
	}

	return fmt.Errorf("gpt360 error: [%s] %s", e.Code, e.Message)
}

func tryParseErrorResponse(body io.Reader) ErrorResponse {
	var errResp struct {
		Error ErrorResponse `json:"error"`
	}

	if err := json.NewDecoder(body).Decode(&errResp); err != nil {
		return ErrorResponse{}
	}

	return errResp.Error
}
