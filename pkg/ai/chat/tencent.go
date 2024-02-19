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
		contextMessages = append(tencentai.Messages{systemMessages[0]}, contextMessages...)
	}

	return tencentai.NewRequest(req.Model, contextMessages)
}

func (chat *TencentAIChat) Chat(ctx context.Context, req Request) (*Response, error) {
	stream, err := chat.ChatStream(ctx, req)
	if err != nil {
		return nil, err
	}

	var content string
	for msg := range stream {
		if msg.ErrorCode != "" {
			return nil, fmt.Errorf("%s %s", msg.ErrorCode, msg.Error)
		}

		content += msg.Text
	}

	return &Response{Text: content}, nil
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
