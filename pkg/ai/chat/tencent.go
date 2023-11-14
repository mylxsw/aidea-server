package chat

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/tencentai"
	"strings"
)

type TencentAIChat struct {
	ai *tencentai.TencentAI
}

func NewTencentAIChat(ai *tencentai.TencentAI) *TencentAIChat {
	return &TencentAIChat{ai: ai}
}

func (chat *TencentAIChat) initRequest(req Request) tencentai.Request {
	req.Model = strings.TrimPrefix(req.Model, "腾讯:")

	var systemMessages tencentai.Messages
	var contextMessages tencentai.Messages

	for _, msg := range req.Messages {
		m := tencentai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		if msg.Role == "system" {
			systemMessages = append(systemMessages, m)
		} else {
			contextMessages = append(contextMessages, m)
		}
	}

	contextMessages = contextMessages.Fix()
	if len(systemMessages) > 0 {
		systemMessage := systemMessages[0]
		finalSystemMessages := make(tencentai.Messages, 0)

		systemMessage.Role = "user"
		finalSystemMessages = append(
			finalSystemMessages,
			systemMessage,
			tencentai.Message{
				Role:    "assistant",
				Content: "好的",
			},
		)

		contextMessages = append(finalSystemMessages, contextMessages...)
	}

	return tencentai.NewRequest(contextMessages)
}

func (chat *TencentAIChat) Chat(ctx context.Context, req Request) (*Response, error) {
	res, err := chat.ai.Chat(ctx, chat.initRequest(req))
	if err != nil {
		return nil, err
	}

	if res.Error.Code != 0 {
		return nil, fmt.Errorf("tencent ai chat error: [%d] %s", res.Error.Code, res.Error.Message)
	}

	return &Response{
		Text:         res.Choices[0].Messages.Content,
		InputTokens:  int(res.Usage.PromptTokens),
		OutputTokens: int(res.Usage.CompletionTokens),
	}, nil
}

func (chat *TencentAIChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	tencentReq := chat.initRequest(req)
	stream, err := chat.ai.ChatStream(ctx, tencentReq)
	if err != nil {
		return nil, err
	}

	res := make(chan Response)
	go func() {
		defer close(res)

		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-stream:
				if !ok {
					return
				}

				if data.Error.Code != 0 {
					select {
					case <-ctx.Done():
					case res <- Response{
						Error:     data.Error.Message,
						ErrorCode: fmt.Sprintf("ERR%d", data.Error.Code),
					}:
					}
					return
				}

				select {
				case <-ctx.Done():
				case res <- Response{
					Text:         data.Choices[0].Delta.Content,
					InputTokens:  int(data.Usage.PromptTokens),
					OutputTokens: int(data.Usage.CompletionTokens),
				}:
				}
			}
		}
	}()

	return res, nil
}

func (chat *TencentAIChat) MaxContextLength(model string) int {
	// https://cloud.tencent.com/document/product/1729/97732
	return 3000
}
