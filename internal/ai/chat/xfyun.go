package chat

import (
	"context"
	"fmt"
	"strings"

	oai "github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/aidea-server/internal/ai/xfyun"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
)

type XFYunChat struct {
	client *xfyun.XFYunAI
}

func NewXFYunChat(client *xfyun.XFYunAI) *XFYunChat {
	return &XFYunChat{client: client}
}

func (chat *XFYunChat) initRequest(req Request) (string, []xfyun.Message, error) {
	req.Model = strings.TrimPrefix(req.Model, "讯飞星火:")

	var systemMessages []openai.ChatCompletionMessage
	var contextMessages []openai.ChatCompletionMessage

	for _, msg := range req.Messages {
		m := openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		if msg.Role == "system" {
			systemMessages = append(systemMessages, m)
		} else {
			contextMessages = append(contextMessages, m)
		}
	}

	msgs, _, err := oai.ReduceChatCompletionMessages(
		contextMessages,
		req.Model,
		8000,
	)
	if err != nil {
		return req.Model, nil, err
	}

	if len(systemMessages) == 1 {
		systemMessages = append(systemMessages, openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: "OK",
		})
	}

	messages := append(systemMessages, msgs...)

	return req.Model, array.Map(messages, func(item openai.ChatCompletionMessage, _ int) xfyun.Message {
		if item.Role == "system" {
			return xfyun.Message{
				Role:    xfyun.RoleUser,
				Content: item.Content,
			}
		}

		return xfyun.Message{
			Role:    xfyun.Role(item.Role),
			Content: item.Content,
		}
	}), nil
}

func (chat *XFYunChat) Chat(ctx context.Context, req Request) (*Response, error) {
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

func (chat *XFYunChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	model, messages, err := chat.initRequest(req)
	if err != nil {
		return nil, err
	}

	stream, err := chat.client.ChatStream(xfyun.Model(model), messages)
	if err != nil {
		return nil, err
	}

	res := make(chan Response)
	go func() {
		defer close(res)

		for data := range stream {
			if data.Header.Code != 0 {
				res <- Response{
					Error:     data.Header.Message,
					ErrorCode: fmt.Sprintf("ERR%d", data.Header.Code),
				}
				return
			}

			res <- Response{
				Text:         data.Payload.Choices.Text[0].Content,
				InputTokens:  data.Payload.Usage.Text.PromptTokens,
				OutputTokens: data.Payload.Usage.Text.CompletionTokens,
			}
		}
	}()

	return res, nil
}
