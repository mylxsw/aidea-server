package sensenova

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/mylxsw/asteria/log"
	"io"
	"net/http"
	"strings"
	"time"
)

type Model string

const (
	// ModelNovaPtcXLV1 官方大语言模型（通用），大参数量
	ModelNovaPtcXLV1 Model = "nova-ptc-xl-v1"
	// ModelNovaPtcXSV1 官方大语言模型（通用），小参数量
	ModelNovaPtcXSV1 Model = "nova-ptc-xs-v1"
)

var (
	// ErrContextExceedLimit 上下文长度超过限制
	ErrContextExceedLimit = fmt.Errorf("context exceed limit")
	ErrSensitivityWord    = fmt.Errorf("sensitivity")
)

type SenseNova struct {
	keyID     string
	keySecret string
}

func New(keyID string, keySecret string) *SenseNova {
	return &SenseNova{
		keyID:     keyID,
		keySecret: keySecret,
	}
}

type Message struct {
	// Role user/assistant/system 消息作者的角色，枚举值。请注意，数组中最后一项必须为 user
	Role string `json:"role,omitempty"`
	// Content 消息的内容
	Content string `json:"content,omitempty"`
}

type Request struct {
	Model Model `json:"model,omitempty"`
	// MaxNewTokens 期望模型生成的最大token数 [1,2048], 默认为 1024
	MaxNewTokens int `json:"max_new_tokens,omitempty"`
	// Messages 输入给模型的对话上下文，数组中的每个对象为聊天的上下文信息
	Messages []Message `json:"messages,omitempty"`
	// Stream 是否使用流式传输，如果开启，数据将按照data-only SSE（server-sent events）返回中间结果，并以 data: [DONE] 结束
	Stream bool `json:"stream,omitempty"`
}

type Response struct {
	Data  RespData  `json:"data,omitempty"`
	Error RespError `json:"error,omitempty"`
}

type RespError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type RespData struct {
	ID      string    `json:"id,omitempty"`
	Choices []Choice  `json:"choices,omitempty"`
	Usage   RespUsage `json:"usage,omitempty"`
}

type Choice struct {
	// Message 非流式请求时，生成的回复内容
	Message string `json:"message,omitempty"`
	// FinishReason 停止生成的原因，枚举值
	// 	因结束符停止生成：stop
	// 	因达到最大生成长度停止生成：length
	// 	因触发敏感词停止生成： sensitive
	// 	因触发模型上下文长度限制： context
	FinishReason string `json:"finish_reason,omitempty"`
	// Delta 流式请求时，生成的回复内容
	Delta string `json:"delta,omitempty"`
}

// RespUsage 本次请求的算法资源使用情况
type RespUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

func (sn *SenseNova) buildToken() string {
	payload := jwt.MapClaims{
		"iss": sn.keyID,
		"exp": time.Now().Add(1800 * time.Second).Unix(),
		"nbf": time.Now().Add(-5 * time.Second).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	signedToken, err := token.SignedString([]byte(sn.keySecret))
	if err != nil {
		log.Errorf("error encoding JWT token for SenseNova: %v", err)
		return ""
	}

	return signedToken
}

// Chat 发起对话
func (sn *SenseNova) Chat(ctx context.Context, req Request) (*Response, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.sensenova.cn/v1/llm/chat-completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+sn.buildToken())
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

		return nil, fmt.Errorf("sensenova chat failed, status [%d], code [%d]: %s", httpResp.StatusCode, errResponse.Code, errResponse.Message)
	}

	var chatResp Response
	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	return &chatResp, nil
}

// ErrorResponse 错误相应 https://platform.sensenova.cn/#/doc?path=/overview/ErrorCode.md
type ErrorResponse struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Details any    `json:"details,omitempty"`
}

func (e ErrorResponse) Error() error {
	if e.Code == 0 {
		return nil
	}

	switch e.Code {
	case 17:
		return ErrContextExceedLimit
	case 18:
		return ErrSensitivityWord
	}

	return fmt.Errorf("sensenova error: [%d] %s", e.Code, e.Message)
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

func (sn *SenseNova) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.sensenova.cn/v1/llm/chat-completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request failed: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+sn.buildToken())
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

		return nil, fmt.Errorf("sensenova chat failed, status [%d], code [%d]: %s", httpResp.StatusCode, errResponse.Code, errResponse.Message)
	}

	res := make(chan Response)
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
				case res <- Response{
					Error: RespError{
						Code:    100,
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
				//id:1
				//event:result
				//data:...
				continue
			}

			if dataStr[5:] == "[DONE]" {
				return
			}

			var chatResponse Response
			if err := json.Unmarshal([]byte(dataStr[5:]), &chatResponse); err != nil {
				select {
				case <-ctx.Done():
				case res <- Response{
					Error: RespError{
						Code:    101,
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
