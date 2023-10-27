package tencentai

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
)

const ModelHyllm = "hyllm"

type TencentAI struct {
	appID     int
	secretID  string
	secretKey string
}

func New(appid int, secretID, secretKey string) *TencentAI {
	return &TencentAI{appID: appid, secretID: secretID, secretKey: secretKey}
}

// Chat 发起一个 Chat
func (ai *TencentAI) Chat(ctx context.Context, req Request) (*Response, error) {
	req.Stream = 0
	req.AppID = ai.appID
	req.SecretID = ai.secretID
	sign := ai.sign(ai.buildSignURL(req))

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %s", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://hunyuan.cloud.tencent.com/hyllm/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request failed: %s", err)
	}

	httpReq.Header.Set("Authorization", sign)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chat failed: %s", err)
	}

	defer httpResp.Body.Close()

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("chat failed [%d]: %s", httpResp.StatusCode, httpResp.Status)
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %s", err)
	}

	respBody = []byte(strings.TrimPrefix(string(respBody), "data: "))

	var chatResp Response
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("decode response failed: %s", err)
	}

	return &chatResp, nil
}

func (ai *TencentAI) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	req.Stream = 1
	req.AppID = ai.appID
	req.SecretID = ai.secretID
	sign := ai.sign(ai.buildSignURL(req))

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://hunyuan.cloud.tencent.com/hyllm/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	log.With(req).Debug("tencent chat stream request")

	httpReq.Header.Set("Authorization", sign)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		_ = httpResp.Body.Close()
		return nil, fmt.Errorf("chat failed [%d]: %s", httpResp.StatusCode, httpResp.Status)
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
				case res <- Response{Error: ResponseError{Message: fmt.Sprintf("read stream data failed: %v", err), Code: 500}}:
				}
				return
			}

			dataStr := strings.TrimSpace(string(data))
			if dataStr == "" {
				continue
			}

			if !strings.HasPrefix(dataStr, "data: ") {
				//id:1
				//event:result
				//data:...
				continue
			}

			var chatResponse Response
			if err := json.Unmarshal([]byte(dataStr[6:]), &chatResponse); err != nil {
				select {
				case <-ctx.Done():
				case res <- Response{Error: ResponseError{Message: fmt.Sprintf("decode response failed: %v", err), Code: 500}}:
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case res <- chatResponse:
				if chatResponse.Choices[0].FinishReason == "stop" {
					return
				}
			}
		}
	}()

	return res, nil
}

// sign 对 url 进行签名
func (ai *TencentAI) sign(url string) string {
	hmacVal := hmac.New(sha1.New, []byte(ai.secretKey))
	signURL := url
	hmacVal.Write([]byte(signURL))
	encryptedStr := hmacVal.Sum([]byte(nil))
	return base64.StdEncoding.EncodeToString(encryptedStr)
}

// buildSignURL 构建签名 URL
func (ai *TencentAI) buildSignURL(req Request) string {
	params := make([]string, 0)
	params = append(params, "app_id="+strconv.Itoa(req.AppID))
	params = append(params, "secret_id="+req.SecretID)
	params = append(params, "timestamp="+strconv.Itoa(req.Timestamp))
	params = append(params, "query_id="+req.QueryID)
	params = append(params, "temperature="+strconv.FormatFloat(req.Temperature, 'f', -1, 64))
	params = append(params, "top_p="+strconv.FormatFloat(req.TopP, 'f', -1, 64))
	params = append(params, "stream="+strconv.Itoa(req.Stream))
	params = append(params, "expired="+strconv.Itoa(req.Expired))

	var messageStr string
	for _, msg := range req.Messages {
		messageStr += fmt.Sprintf(`{"role":"%s","content":"%s"},`, msg.Role, msg.Content)
	}
	messageStr = strings.TrimSuffix(messageStr, ",")
	params = append(params, "messages=["+messageStr+"]")

	sort.Strings(params)
	return "hunyuan.cloud.tencent.com/hyllm/v1/chat/completions?" + strings.Join(params, "&")
}

func NewRequest(messages []Message) Request {
	queryID, _ := uuid.GenerateUUID()
	return Request{
		Timestamp:   int(time.Now().Unix()),
		Expired:     int(time.Now().Unix()) + 24*60*60,
		Temperature: 0,
		TopP:        0.8,
		Messages:    messages,
		QueryID:     queryID,
	}
}

type Request struct {
	// AppID 腾讯云账号的 APPID
	AppID int `json:"app_id"`
	// SecretID 官网 SecretId
	SecretID string `json:"secret_id"`
	QueryID  string `json:"query_id"`
	// Timestamp 当前 UNIX 时间戳，单位为秒，可记录发起 API 请求的时间。
	// 例如1529223702，如果与当前时间相差过大，会引起签名过期错误
	Timestamp int `json:"timestamp"`
	// Expired 签名的有效期，是一个符合 UNIX Epoch 时间戳规范的数值，单位为秒；
	// Expired 必须大于 Timestamp 且 Expired-Timestamp 小于90天
	Expired int `json:"expired"`
	// Temperature 较高的数值会使输出更加随机，而较低的数值会使其更加集中和确定
	// 默认1.0，取值区间为[0.0, 2.0]，非必要不建议使用, 不合理的取值会影响效果
	// 建议该参数和top_p只设置1个，不要同时更改 top_p
	Temperature float64 `json:"temperature"`
	// TopP 影响输出文本的多样性，取值越大，生成文本的多样性越强
	// 默认1.0，取值区间为[0.0, 1.0]，非必要不建议使用, 不合理的取值会影响效果
	// 建议该参数和 temperature 只设置1个，不要同时更改
	TopP float64 `json:"top_p"`
	// Stream 是否流式输出 1：流式 0：同步
	// 注意 ：同步模式和流式模式，响应参数返回不同;
	// 同步请求超时时间为60s，如果内容较长请使用流式模式
	// 同步模式：响应参数为完整 json 包
	// 流式模式：响应参数为 data: {响应参数}
	Stream int `json:"stream"`
	// Messages 会话内容, 长度最多为40, 按对话时间从旧到新在数组中排列
	// 输入 content 总数最大支持 3000 token。
	Messages Messages `json:"messages"`
}

type Message struct {
	// Role 当前支持以下：
	// user：表示用户
	// assistant：表示对话助手
	// 在 message 中必须是 user 与 assistant 交替(一问一答)
	Role string `json:"role"`
	// Content 消息的内容
	Content string `json:"content"`
}

type Messages []Message

func (ms Messages) Fix() Messages {
	last := ms[len(ms)-1]
	if last.Role != "user" {
		last = Message{
			Role:    "user",
			Content: "继续",
		}
		ms = append(ms, last)
	}

	finalMessages := make([]Message, 0)
	var lastRole string

	for _, m := range array.Reverse(ms) {
		if m.Role == lastRole {
			continue
		}

		lastRole = m.Role
		finalMessages = append(finalMessages, m)
	}

	if len(finalMessages)%2 == 0 {
		finalMessages = finalMessages[:len(finalMessages)-1]
	}

	return array.Reverse(finalMessages)
}

type Response struct {
	// Choices 结果
	Choices []ResponseChoices `json:"choices,omitempty"`
	// Created unix 时间戳的字符串
	Created string `json:"created,omitempty"`
	// ID 会话 id
	ID string `json:"id,omitempty"`
	// Usage token 数量
	Usage ResponseUsage `json:"usage,omitempty"`
	// Error 错误信息
	// 注意：此字段可能返回 null，表示取不到有效值
	Error ResponseError `json:"error,omitempty"`
	// Note 注释
	Note string `json:"note,omitempty"`
	// ReqID 唯一请求 ID，每次请求都会返回。用于反馈接口入参
	ReqID string `json:"req_id,omitempty"`
}

type ResponseChoices struct {
	// FinishReason 流式结束标志位，为 stop 则表示尾包
	FinishReason string `json:"finish_reason,omitempty"`
	// Message 内容，同步模式返回内容，流模式为 null
	// 输出 content 内容总数最多支持 1024token
	Messages Message `json:"messages,omitempty"`
	// Delta 内容，流模式返回内容，同步模式为 null
	// 输出 content 内容总数最多支持 1024token。
	Delta Message `json:"delta,omitempty"`
}

type ResponseUsage struct {
	// PromptTokens 输入 token 数量
	PromptTokens int64 `json:"prompt_tokens,omitempty"`
	// TotalTokens 总 token 数量
	TotalTokens int64 `json:"total_tokens,omitempty"`
	// CompletionTokens 输出 token 数量
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
}

type ResponseError struct {
	// Message 错误提示信息
	Message string `json:"message,omitempty"`
	// Code 错误码
	Code int `json:"code,omitempty"`
}
