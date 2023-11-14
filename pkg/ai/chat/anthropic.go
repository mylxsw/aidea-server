package chat

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/anthropic"
	"strings"
)

type AnthropicChat struct {
	ai *anthropic.Anthropic
}

func NewAnthropicChat(ai *anthropic.Anthropic) *AnthropicChat {
	return &AnthropicChat{ai: ai}
}

func (chat *AnthropicChat) initRequest(req Request) anthropic.Request {
	req.Model = strings.TrimPrefix(req.Model, "Anthropic:")

	var systemMessages anthropic.Messages
	var contextMessages anthropic.Messages

	for _, msg := range req.Messages {
		m := anthropic.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		if msg.Role == "system" {
			systemMessages = append(systemMessages, m)
		} else {
			contextMessages = append(contextMessages, m)
		}
	}

	if len(systemMessages) > 0 {
		systemMessage := systemMessages[0]
		finalSystemMessages := make(anthropic.Messages, 0)

		systemMessage.Role = "user"
		finalSystemMessages = append(
			finalSystemMessages,
			systemMessage,
			anthropic.Message{
				Role:    "assistant",
				Content: "好的",
			},
		)

		contextMessages = append(finalSystemMessages, contextMessages...)
	}

	// Bugfix: prompt must start with "\n\nHuman:" turn
	if len(contextMessages) > 0 && contextMessages[0].Role != "user" {
		contextMessages = contextMessages[1:]
	}

	return anthropic.NewRequest(anthropic.Model(req.Model), contextMessages)
}

func (chat *AnthropicChat) Chat(ctx context.Context, req Request) (*Response, error) {
	res, err := chat.ai.Chat(ctx, chat.initRequest(req))
	if err != nil {
		return nil, err
	}

	if res.Error != nil && res.Error.Type != "" {
		return nil, fmt.Errorf("anthropic ai chat error: [%s] %s", res.Error.Type, res.Error.Message)
	}

	return &Response{
		Text: res.Completion,
	}, nil
}

func (chat *AnthropicChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	stream, err := chat.ai.ChatStream(ctx, chat.initRequest(req))
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
				if data.Error != nil && data.Error.Type != "" {
					select {
					case <-ctx.Done():
					case res <- Response{Error: data.Error.Message, ErrorCode: data.Error.Type}:
					}
					return
				}

				select {
				case <-ctx.Done():
					return
				case res <- Response{Text: data.Completion}:
				}
			}
		}
	}()

	return res, nil
}

func (chat *AnthropicChat) MaxContextLength(model string) int {
	// https://docs.anthropic.com/claude/reference/selecting-a-model
	// 这里减掉 4000 用于输出
	return 100000 - 4000
}
