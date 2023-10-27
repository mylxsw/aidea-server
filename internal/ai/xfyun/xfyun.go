package xfyun

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Model string

const (
	ModelGeneralV1_5 Model = "general"
	ModelGeneralV2   Model = "generalv2"
	ModelGeneralV3   Model = "generalv3"
)

type XFYunAI struct {
	appID     string
	apiKey    string
	apiSecret string
}

func New(appID string, apiKey, apiSecret string) *XFYunAI {
	return &XFYunAI{appID: appID, apiKey: apiKey, apiSecret: apiSecret}
}

type Response struct {
	Header  ResponseHeader  `json:"header,omitempty"`
	Payload ResponsePayload `json:"payload,omitempty"`
}

type ResponsePayload struct {
	Choices PayloadChoice `json:"choices,omitempty"`
	Usage   PayloadUsage  `json:"usage,omitempty"`
}

type PayloadUsage struct {
	Text struct {
		// QuestionTokens 保留字段，可忽略
		QuestionTokens int `json:"question_tokens,omitempty"`
		// PromptTokens 包含历史问题的总tokens大小
		PromptTokens int `json:"prompt_tokens,omitempty"`
		// CompletionTokens 回答的tokens大小
		CompletionTokens int `json:"completion_tokens,omitempty"`
		// TotalTokens prompt_tokens和completion_tokens的和，也是本次交互计费的tokens大小
		TotalTokens int `json:"total_tokens,omitempty"`
	} `json:"text,omitempty"`
}

type PayloadChoice struct {
	// Status 文本响应状态，取值为[0,1,2]; 0代表首个文本结果；1代表中间文本结果；2代表最后一个文本结果
	Status int `json:"status,omitempty"`
	// Seq 返回的数据序号，取值为[0,9999999]
	Seq  int                 `json:"seq,omitempty"`
	Text []PayloadChoiceText `json:"text,omitempty"`
}

type PayloadChoiceText struct {
	// Content AI的回答内容
	Content string `json:"content,omitempty"`
	// Role 角色标识，固定为assistant，标识角色为AI
	Role Role `json:"role,omitempty"`
	// Index 结果序号，取值为[0,10]; 当前为保留字段，开发者可忽略
	Index int `json:"index,omitempty"`
}

type ResponseHeader struct {
	// Code 错误码，0表示正常，非0表示出错；详细释义可在接口说明文档最后的错误码说明了解
	Code int `json:"code,omitempty"`
	// Message 会话是否成功的描述信息
	Message string `json:"message,omitempty"`
	// SID 会话的唯一id，用于讯飞技术人员查询服务端会话日志使用,出现调用错误时建议留存该字段
	SID string `json:"sid,omitempty"`
	// Status 会话状态，取值为[0,1,2]；0代表首次结果；1代表中间结果；2代表最后一个结果
	Status int `json:"status,omitempty"`
}

// ChatStream 发起聊天
func (ai *XFYunAI) ChatStream(ctx context.Context, model Model, messages []Message) (<-chan Response, error) {
	ws := websocket.DefaultDialer

	host := ai.resolveHostForModel(model)
	urlStr := ai.assembleAuthURL(host, ai.apiKey, ai.apiSecret)
	conn, resp, err := ws.DialContext(ctx, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 WS 连接失败：%w [%d %s]", err, resp.StatusCode, resp.Status)
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("创建 WS 连接失败，状态码：%d", resp.StatusCode)
	}

	req := ai.buildParams(model, messages)
	if err := conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("发送消息失败：%w", err)
	}

	respChan := make(chan Response)
	go func() {
		defer func() {
			close(respChan)
			_ = conn.Close()
		}()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if err == io.EOF || websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}

				select {
				case <-ctx.Done():
				case respChan <- Response{Header: ResponseHeader{Code: -1, Message: err.Error()}}:
				}
				return
			}

			var ret Response
			if err := json.Unmarshal(msg, &ret); err != nil {
				select {
				case <-ctx.Done():
				case respChan <- Response{Header: ResponseHeader{Code: -1, Message: err.Error()}}:
				}
				return
			}

			if ret.Header.Code != 0 {
				select {
				case <-ctx.Done():
				case respChan <- ret:
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case respChan <- ret:
			}
		}
	}()

	return respChan, nil
}

// assembleAuthURL 组装鉴权 URL
func (ai *XFYunAI) assembleAuthURL(host, apiKey, secret string) string {
	ul, err := url.Parse(host)
	if err != nil {
		panic(err)
	}

	date := time.Now().UTC().Format(time.RFC1123)
	signString := []string{"host: " + ul.Host, "date: " + date, "GET " + ul.Path + " HTTP/1.1"}
	sign := strings.Join(signString, "\n")
	sha := ai.hmacWithShaToBase64(sign, secret)

	authUrl := fmt.Sprintf(`hmac username="%s", algorithm="%s", headers="%s", signature="%s"`,
		apiKey,
		"hmac-sha256",
		"host date request-line",
		sha,
	)
	authorization := base64.StdEncoding.EncodeToString([]byte(authUrl))

	v := url.Values{}
	v.Add("host", ul.Host)
	v.Add("date", date)
	v.Add("authorization", authorization)

	return host + "?" + v.Encode()
}

func (ai *XFYunAI) hmacWithShaToBase64(data, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Message 聊天上下文信息
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// buildParams 构建请求参数
func (ai *XFYunAI) buildParams(model Model, messages []Message) map[string]any {
	data := map[string]any{
		"header": map[string]any{
			"app_id": ai.appID,
		},
		"parameter": map[string]any{
			"chat": map[string]any{
				"domain":      model,
				"temperature": 0.8,
				"top_k":       int64(6),
				"max_tokens":  int64(2048),
				"auditing":    "default",
			},
		},
		"payload": map[string]any{
			"message": map[string]any{
				"text": messages,
			},
		},
	}

	return data
}

// resolveResponse 解析响应
func (ai *XFYunAI) resolveResponse(resp *http.Response) (string, error) {
	if resp == nil {
		return "", nil
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("code=%d,body=%s", resp.StatusCode, string(b)), nil
}

// resolveHostForModel 根据模型获取对应的 host
func (ai *XFYunAI) resolveHostForModel(model Model) string {
	if model == ModelGeneralV1_5 {
		return "wss://spark-api.xf-yun.com/v1.1/chat"
	}

	if model == ModelGeneralV3 {
		return "wss://spark-api.xf-yun.com/v3.1/chat"
	}

	return "wss://spark-api.xf-yun.com/v2.1/chat"
}
