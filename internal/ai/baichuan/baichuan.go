package baichuan

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	ModelBaichuan2_53B = "Baichuan2-53B"
)

type BaichuanAI struct {
	apiKey    string
	apiSecret string
}

func NewBaichuanAI(apiKey, apiSecret string) *BaichuanAI {
	return &BaichuanAI{apiKey: apiKey, apiSecret: apiSecret}
}

type Request struct {
	// Model 使用的模型 ID，当前默认 Baichuan2-53B
	Model string `json:"model"`
	// Messages 对话消息列表 (历史对话按从老到新顺序填入)
	Messages   []Message  `json:"messages"`
	Parameters Parameters `json:"parameters"`
}

type Parameters struct {
	// Temperature 取值范围: [.0f, 1.0f]。 多样性，越高，多样性越好, 缺省 0.3
	Temperature float64 `json:"temperature,omitempty"`
	// TopK 取值范围: [0, 20]。搜索采样控制参数，越大，采样集大, 0 则不走 top_k 采样筛选策略，最大 20(超过 20 会被修正成 20)，缺省 5
	TopK int `json:"top_k,omitempty"`
	// TopP 取值范围: [.0f, 1.0f)。值越小，越容易出头部, 缺省 0.85
	TopP float64 `json:"top_p,omitempty"`
	// WithSearchEnhance 开启搜索增强，搜索增强会产生额外的费用, 缺省 False
	WithSearchEnhance bool `json:"with_search_enhance,omitempty"`
}

type Message struct {
	// Role user=用户, assistant=助手
	Role string `json:"role"`
	// Content 内容
	Content string `json:"content"`
}

type Response struct {
	// Code 状态码，0 表示成功，非 0 表示失败
	Code int `json:"code"`
	// Message 提示信息
	Message string `json:"msg,omitempty"`
	// Data 对话结果
	Data ResponseData `json:"data,omitempty"`
	// Usage token 使用信息
	Usage ResponseUsage `json:"usage,omitempty"`
}

type ResponseData struct {
	Messages []ResponseMessage `json:"messages,omitempty"`
}

type ResponseMessage struct {
	// Role user=用户, assistant=助手
	Role string `json:"role,omitempty"`
	// Content 内容
	Content string `json:"content,omitempty"`
	// FinishReason 会话终止原因
	FinishReason string `json:"finish_reason,omitempty"`
}

type ResponseUsage struct {
	// PromptTokens prompt 的 token 数
	PromptTokens int `json:"prompt_tokens,omitempty"`
	// AnswerTokens 回答生成的 token 数
	AnswerTokens int `json:"answer_tokens,omitempty"`
	// TotalTokens 会话的总 token 数
	TotalTokens int `json:"total_tokens,omitempty"`
}

func (ai *BaichuanAI) Chat(ctx context.Context, req Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %s", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.baichuan-ai.com/v1/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request failed: %s", err)
	}

	for k, v := range ai.buildHeaders(string(body)) {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chat failed: %s", err)
	}

	defer httpResp.Body.Close()

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("chat failed [%s]: %s", httpResp.Status, string(data))
	}

	var chatResp Response
	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response failed: %s", err)
	}

	return &chatResp, nil
}

func (ai *BaichuanAI) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.baichuan-ai.com/v1/stream/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	for k, v := range ai.buildHeaders(string(body)) {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()

		return nil, fmt.Errorf("chat failed [%s]: %s", httpResp.Status, string(data))
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
				case res <- Response{Code: 100, Message: fmt.Sprintf("read response failed: %v", err)}:
				}
				return
			}

			dataStr := strings.TrimSpace(string(data))
			if dataStr == "" {
				continue
			}

			var chatResponse Response
			if err := json.Unmarshal([]byte(dataStr), &chatResponse); err != nil {
				select {
				case <-ctx.Done():
				case res <- Response{Code: 100, Message: fmt.Sprintf("decode response failed: %v", err)}:
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case res <- chatResponse:
				for _, msg := range chatResponse.Data.Messages {
					if msg.FinishReason != "" {
						return
					}
				}
			}
		}
	}()

	return res, nil
}

func (ai *BaichuanAI) buildSignature(body string) (sign string, timestamp int64) {
	// 鉴权涉及的参数描述：
	// SecretKey：      与APIKey唯一对应的私钥，由百川提供
	// HTTP-Body：      客户端发送POST请求的请求体
	// X-BC-Timestamp： UTC标准时间戳，例如 1692950259
	//
	// 客户端请求签名的计算规则：
	// X-BC-Signature = md5(SecretKey + HTTP-Body + X-BC-Timestamp)
	timestamp = time.Now().Unix()
	return fmt.Sprintf("%x", md5.Sum([]byte(ai.apiSecret+body+strconv.Itoa(int(timestamp))))), timestamp
}

func (ai *BaichuanAI) buildHeaders(body string) (headers map[string]string) {
	sign, timestamp := ai.buildSignature(body)
	return map[string]string{
		"X-BC-Signature": sign,
		"X-BC-Timestamp": strconv.Itoa(int(timestamp)),
		"Content-Type":   "application/json",
		"Authorization":  "Bearer " + ai.apiKey,
		"X-BC-Sign-Algo": "MD5",
	}
}
