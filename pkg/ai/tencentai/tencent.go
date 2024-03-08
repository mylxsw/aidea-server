package tencentai

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/go-utils/array"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	v20230901 "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/hunyuan/v20230901"
)

const ModelHyllm = "hyllm"
const ModelHyllmStd = "hyllm_std"
const ModelHyllmPro = "hyllm_pro"

type TencentAI struct {
	client *v20230901.Client
}

func New(secretID, secretKey string) *TencentAI {
	client, _ := v20230901.NewClient(
		common.NewCredential(secretID, secretKey),
		regions.Guangzhou,
		profile.NewClientProfile(),
	)
	return &TencentAI{
		client: client,
	}
}

func (ai *TencentAI) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	switch req.Model {
	case ModelHyllmPro:
		return ai.chatProStream(ctx, req)
	case ModelHyllmStd:
		return ai.chatStdStream(ctx, req)
	case ModelHyllm:
		return ai.chatStdStream(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported model: %s", req.Model)
	}
}

func (ai *TencentAI) chatStdStream(ctx context.Context, req Request) (<-chan Response, error) {
	stdReq := v20230901.NewChatStdRequest()

	if req.TopP != 0 {
		stdReq.TopP = &req.TopP
	}

	if req.Temperature != 0 {
		stdReq.Temperature = &req.Temperature
	}

	stdReq.Messages = array.Map(req.Messages, func(m Message, _ int) *v20230901.Message {
		return &v20230901.Message{
			Role:    common.StringPtr(m.Role),
			Content: common.StringPtr(m.Content),
		}
	})

	ctx, cancel := context.WithCancel(ctx)
	resp, err := ai.client.ChatStdWithContext(ctx, stdReq)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("tencent chat hyllm std failed: %w", err)
	}

	res := make(chan Response)
	go func() {
		defer func() {
			cancel()
			close(res)
		}()

		for event := range resp.Events {
			if event.Err != nil {
				select {
				case <-ctx.Done():
				case res <- Response{Error: ResponseError{Message: event.Err.Error(), Code: 500}}:
				}
				return
			}

			var chatResponse Response
			if err := json.Unmarshal(event.Data, &chatResponse); err != nil {
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

func (ai *TencentAI) chatProStream(ctx context.Context, req Request) (<-chan Response, error) {
	proReq := v20230901.NewChatProRequest()

	if req.TopP != 0 {
		proReq.TopP = &req.TopP
	}

	if req.Temperature != 0 {
		proReq.Temperature = &req.Temperature
	}

	proReq.Messages = array.Map(req.Messages, func(m Message, _ int) *v20230901.Message {
		return &v20230901.Message{
			Role:    common.StringPtr(m.Role),
			Content: common.StringPtr(m.Content),
		}
	})

	ctx, cancel := context.WithCancel(ctx)
	resp, err := ai.client.ChatProWithContext(ctx, proReq)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("tencent chat hyllm std failed: %w", err)
	}

	res := make(chan Response)
	go func() {
		defer func() {
			cancel()
			close(res)
		}()

		for event := range resp.Events {

			if event.Err != nil {
				select {
				case <-ctx.Done():
				case res <- Response{Error: ResponseError{Message: event.Err.Error(), Code: 500}}:
				}
				return
			}

			var chatResponse Response
			if err := json.Unmarshal(event.Data, &chatResponse); err != nil {
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

func NewRequest(model string, messages []Message) Request {
	return Request{
		Model:       model,
		Temperature: 0,
		TopP:        0.8,
		Messages:    messages,
	}
}

type Request struct {
	// Model 模型
	Model string `json:"-"`
	// Temperature 较高的数值会使输出更加随机，而较低的数值会使其更加集中和确定
	// 默认1.0，取值区间为[0.0, 2.0]，非必要不建议使用, 不合理的取值会影响效果
	// 建议该参数和top_p只设置1个，不要同时更改 top_p
	Temperature float64 `json:"temperature"`
	// TopP 影响输出文本的多样性，取值越大，生成文本的多样性越强
	// 默认1.0，取值区间为[0.0, 1.0]，非必要不建议使用, 不合理的取值会影响效果
	// 建议该参数和 temperature 只设置1个，不要同时更改
	TopP float64 `json:"top_p"`
	// Messages 会话内容, 长度最多为40, 按对话时间从旧到新在数组中排列
	// 输入 content 总数最大支持 3000 token。
	Messages Messages `json:"messages"`
}

type Message struct {
	// Role 当前支持以下：
	// system: 系统提示语，必须为第一个
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
	// ID 会话 id
	ID string `json:"id,omitempty"`
	// Usage token 数量
	Usage ResponseUsage `json:"usage,omitempty"`
	// Error 错误信息
	// 注意：此字段可能返回 null，表示取不到有效值
	Error ResponseError `json:"error,omitempty"`
	// Note 注释
	Note string `json:"note,omitempty"`
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
