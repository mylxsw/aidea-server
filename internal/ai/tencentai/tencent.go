package tencentai

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
)

type TencentAI struct {
	appID  string
	secret string
}

func New(appid, secret string) *TencentAI {
	return &TencentAI{appID: appid, secret: secret}
}

// sign 对 url 进行签名
func (ai *TencentAI) sign(url string) string {
	hmac := hmac.New(sha1.New, []byte(ai.secret))
	signURL := url
	hmac.Write([]byte(signURL))
	encryptedStr := hmac.Sum([]byte(nil))
	var signature = base64.StdEncoding.EncodeToString(encryptedStr)
	return signature
}

func (ai *TencentAI) buildSignURL(req Request) string {
	params := make([]string, 0)
	params = append(params, "app_id="+string(req.AppID))
	params = append(params, "secret_id="+string(req.SecretID))
	params = append(params, "timestamp="+string(req.Timestamp))

	return ""
}

type Request struct {
	// AppID 腾讯云账号的 APPID
	AppID int64 `json:"app_id,omitempty"`
	// SecretID 官网 SecretId
	SecretID string `json:"secret_id,omitempty"`
	// Timestamp 当前 UNIX 时间戳，单位为秒，可记录发起 API 请求的时间。
	// 例如1529223702，如果与当前时间相差过大，会引起签名过期错误
	Timestamp int64 `json:"timestamp,omitempty"`
	// Expired 签名的有效期，是一个符合 UNIX Epoch 时间戳规范的数值，单位为秒；
	// Expired 必须大于 Timestamp 且 Expired-Timestamp 小于90天
	Expired int64 `json:"expired,omitempty"`
	// Temperature 较高的数值会使输出更加随机，而较低的数值会使其更加集中和确定
	// 默认1.0，取值区间为[0.0, 2.0]，非必要不建议使用, 不合理的取值会影响效果
	// 建议该参数和top_p只设置1个，不要同时更改 top_p
	Temperature float64 `json:"temperature,omitempty"`
	// TopP 影响输出文本的多样性，取值越大，生成文本的多样性越强
	// 默认1.0，取值区间为[0.0, 1.0]，非必要不建议使用, 不合理的取值会影响效果
	// 建议该参数和 temperature 只设置1个，不要同时更改
	TopP float64 `json:"top_p,omitempty"`
	// Stream 是否流式输出 1：流式 0：同步
	// 注意 ：同步模式和流式模式，响应参数返回不同;
	// 同步请求超时时间为60s，如果内容较长请使用流式模式
	// 同步模式：响应参数为完整 json 包
	// 流式模式：响应参数为 data: {响应参数}
	Stream int64 `json:"stream,omitempty"`
	// Messages 会话内容, 长度最多为40, 按对话时间从旧到新在数组中排列
	// 输入 content 总数最大支持 3000 token。
	Messages []Message `json:"messages,omitempty"`
}

type Message struct {
	// Role 当前支持以下：
	// user：表示用户
	// assistant：表示对话助手
	// 在 message 中必须是 user 与 assistant 交替(一问一答)
	Role string `json:"role,omitempty"`
	// Content 消息的内容
	Content string `json:"content,omitempty"`
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
	Delta string `json:"delta,omitempty"`
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
	// Mesasge 错误提示信息
	Message string `json:"message,omitempty"`
	// Code 错误码
	Code string `json:"code,omitempty"`
}
