package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mylxsw/aidea-server/internal/ai/sensenova"
	"github.com/mylxsw/go-utils/array"
)

type SenseNovaChat struct {
	sensenova *sensenova.SenseNova
}

func NewSenseNovaChat(sensenova *sensenova.SenseNova) *SenseNovaChat {
	return &SenseNovaChat{sensenova: sensenova}
}

func (ds *SenseNovaChat) initRequest(req Request) sensenova.Request {
	req.Messages = req.Messages.Fix()

	messages := array.Map(req.Messages, func(item Message, _ int) sensenova.Message {
		return sensenova.Message{
			Role:    item.Role,
			Content: item.Content,
		}
	})

	return sensenova.Request{
		Model:    sensenova.Model(strings.TrimPrefix(req.Model, "商汤日日新:")),
		Messages: messages,
	}
}

func (ds *SenseNovaChat) Chat(ctx context.Context, req Request) (*Response, error) {
	chatReq := ds.initRequest(req)
	resp, err := ds.sensenova.Chat(ctx, chatReq)
	if err != nil {
		if errors.Is(err, sensenova.ErrContextExceedLimit) {
			return nil, ErrContextExceedLimit
		}

		return nil, err
	}

	if resp.Error.Code != 0 {
		return nil, fmt.Errorf("sensenova chat error: [%d] %s", resp.Error.Code, resp.Error.Message)
	}

	var content string
	var finishReason string
	for _, c := range resp.Data.Choices {
		content += c.Message
		finishReason = c.FinishReason
	}

	return &Response{
		Text:         content,
		FinishReason: finishReason,
		InputTokens:  resp.Data.Usage.PromptTokens,
		OutputTokens: resp.Data.Usage.CompletionTokens,
	}, nil
}

func (ds *SenseNovaChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	stream, err := ds.sensenova.ChatStream(ctx, ds.initRequest(req))
	if err != nil {
		if errors.Is(err, sensenova.ErrContextExceedLimit) {
			return nil, ErrContextExceedLimit
		}

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

				var content string
				var finishReason string
				for _, c := range data.Data.Choices {
					content += c.Delta
					finishReason = c.FinishReason
				}

				select {
				case <-ctx.Done():
					return
				case res <- Response{
					Text:         content,
					FinishReason: finishReason,
					InputTokens:  data.Data.Usage.PromptTokens,
					OutputTokens: data.Data.Usage.CompletionTokens,
				}:
				}
			}
		}
	}()

	return res, nil
}

func (ds *SenseNovaChat) MaxContextLength(model string) int {
	// https://platform.sensenova.cn/#/doc?path=/chat/GetStarted/ModelList.md
	return 2000
}
