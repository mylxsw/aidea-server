package chat

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/gpt360"
	"strings"

	"github.com/mylxsw/go-utils/array"
)

type GPT360Chat struct {
	g360 *gpt360.GPT360
}

func NewGPT360Chat(g360 *gpt360.GPT360) *GPT360Chat {
	return &GPT360Chat{g360: g360}
}

func (ds *GPT360Chat) initRequest(req Request) gpt360.ChatRequest {
	req.Messages = req.Messages.Fix()

	messages := array.Map(req.Messages, func(item Message, _ int) gpt360.Message {
		return gpt360.Message{
			Role:    item.Role,
			Content: item.Content,
		}
	})

	return gpt360.ChatRequest{
		Model:    strings.TrimPrefix(req.Model, "360智脑:"),
		Messages: messages,
	}
}

func (ds *GPT360Chat) Chat(ctx context.Context, req Request) (*Response, error) {
	chatReq := ds.initRequest(req)
	resp, err := ds.g360.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	if resp.Error.Code != "" {
		return nil, fmt.Errorf("gpt360 chat error: [%s] %s", resp.Error.Code, resp.Error.Message)
	}

	var content string
	var finishReason string
	for _, c := range resp.Choices {
		content += c.Message.Content
		finishReason = c.FinishReason
	}

	return &Response{
		Text:         content,
		FinishReason: finishReason,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}, nil
}

func (ds *GPT360Chat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	stream, err := ds.g360.ChatStream(ctx, ds.initRequest(req))
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

				if data.Error.Code != "" {
					select {
					case <-ctx.Done():
					case res <- Response{Error: data.Error.Message, ErrorCode: fmt.Sprintf("ERR%s", data.Error.Code)}:
					}
					return
				}

				var content string
				var finishReason string
				for _, c := range data.Choices {
					content += c.Delta.Content
					finishReason = c.FinishReason
				}

				select {
				case <-ctx.Done():
					return
				case res <- Response{
					Text:         content,
					FinishReason: finishReason,
					InputTokens:  data.Usage.PromptTokens,
					OutputTokens: data.Usage.CompletionTokens,
				}:
				}
			}
		}
	}()

	return res, nil
}

func (ds *GPT360Chat) MaxContextLength(model string) int {
	// https://ai.360.com/platform/docs/overview
	return 2000
}
