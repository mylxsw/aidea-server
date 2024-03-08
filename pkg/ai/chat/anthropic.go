package chat

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/anthropic"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/go-utils/ternary"
	"strings"
)

type AnthropicChat struct {
	ai *anthropic.Anthropic
}

func NewAnthropicChat(ai *anthropic.Anthropic) *AnthropicChat {
	return &AnthropicChat{ai: ai}
}

func (chat *AnthropicChat) initRequest(req Request) anthropic.MessageRequest {
	req.Model = strings.TrimPrefix(req.Model, "Anthropic:")

	var systemMessage string
	var contextMessages []anthropic.Message

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if msg.Content != "" {
				systemMessage = msg.Content
			}
		} else {
			if msg.MultipartContents != nil {
				contents := make([]anthropic.MessageContent, 0)
				for _, ct := range msg.MultipartContents {
					item := anthropic.MessageContent{Type: ternary.If(ct.Type == "text", "text", "image")}
					if ct.Type == "text" {
						item.Text = ct.Text
					} else if ct.ImageURL != nil {
						item.Source = anthropic.NewImageSource(
							misc.Base64ImageMediaType(ct.ImageURL.URL),
							misc.RemoveImageBase64Prefix(ct.ImageURL.URL),
						)
					}

					contents = append(contents, item)
				}

				contextMessages = append(contextMessages, anthropic.Message{
					Role:    msg.Role,
					Content: contents,
				})
			} else {
				contextMessages = append(contextMessages, anthropic.NewTextMessage(msg.Role, msg.Content))
			}
		}
	}

	res := anthropic.MessageRequest{
		Model:    anthropic.Model(req.Model),
		Messages: contextMessages,
	}

	if systemMessage != "" {
		res.System = systemMessage
	}

	return res
}

func (chat *AnthropicChat) Chat(ctx context.Context, req Request) (*Response, error) {
	res, err := chat.ai.Chat(ctx, chat.initRequest(req))
	if err != nil {
		return nil, err
	}

	if res.Error != nil && res.Error.Type != "" {
		return nil, fmt.Errorf("anthropic ai chat error: [%s] %s", res.Error.Type, res.Error.Message)
	}

	ret := Response{Text: res.Text()}
	if res.Usage != nil {
		ret.InputTokens = res.Usage.InputTokens
		ret.OutputTokens = res.Usage.OutputTokens
	}

	return &ret, nil
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
				case res <- Response{Text: data.Text()}:
				}
			}
		}
	}()

	return res, nil
}

func (chat *AnthropicChat) MaxContextLength(model string) int {
	// https://docs.anthropic.com/claude/reference/selecting-a-model
	// 这里减掉 4000 用于输出
	if model == string(anthropic.ModelClaudeInstant) {
		return 100000 - 4096
	}

	return 200000 - 4096
}
