package chat

import (
	"context"
	"fmt"
	"strings"

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

	if len(systemMessages) == 1 {
		systemMessages = append(systemMessages, openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: "OK",
		})
	}

	messages := append(systemMessages, contextMessages...)

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

	stream, err := chat.client.ChatStream(ctx, xfyun.Model(model), messages)
	if err != nil {
		return nil, err
	}

	res := make(chan Response)
	go func() {
		defer close(res)

		for {
			select {
			case <-ctx.Done():
			case data, ok := <-stream:
				if !ok {
					return
				}

				if data.Header.Code != 0 {
					select {
					case <-ctx.Done():
					case res <- Response{
						Error:     data.Header.Message,
						ErrorCode: fmt.Sprintf("ERR%d", data.Header.Code),
					}:
					}
					return
				}

				select {
				case <-ctx.Done():
				case res <- Response{
					Text:         data.Payload.Choices.Text[0].Content,
					InputTokens:  data.Payload.Usage.Text.PromptTokens,
					OutputTokens: data.Payload.Usage.Text.CompletionTokens,
				}:
				}
			}
		}
	}()

	return res, nil
}

func (chat *XFYunChat) MaxContextLength(model string) int {
	// https://www.xfyun.cn/doc/spark/Web.html#_1-%E6%8E%A5%E5%8F%A3%E8%AF%B4%E6%98%8E
	switch xfyun.Model(model) {
	case xfyun.ModelGeneralV2:
		return 8000
	case xfyun.ModelGeneralV1_5:
		return 4000
	}

	return 4000
}
