package chat

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/internal/ai/baichuan"
	"github.com/mylxsw/go-utils/array"
)

type BaichuanAIChat struct {
	ai *baichuan.BaichuanAI
}

func NewBaichuanAIChat(ai *baichuan.BaichuanAI) *BaichuanAIChat {
	return &BaichuanAIChat{ai: ai}
}

func (ai *BaichuanAIChat) initRequest(req Request) baichuan.Request {
	req.Messages = req.Messages.Fix()

	messages := array.Map(req.Messages, func(item Message, _ int) baichuan.Message {
		return baichuan.Message{
			Role:    item.Role,
			Content: item.Content,
		}
	})

	return baichuan.Request{
		Model:    req.Model,
		Messages: messages,
		Parameters: baichuan.Parameters{
			WithSearchEnhance: true,
		},
	}
}

func (ai *BaichuanAIChat) Chat(ctx context.Context, req Request) (*Response, error) {
	chatReq := ai.initRequest(req)
	resp, err := ai.ai.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("baichuan chat error: [%d] %s", resp.Code, resp.Message)
	}

	var content string
	var finishReason string
	for _, c := range resp.Data.Messages {
		content += c.Content
		finishReason = c.FinishReason
	}

	return &Response{
		Text:         content,
		FinishReason: finishReason,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.AnswerTokens,
	}, nil
}

func (ai *BaichuanAIChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	stream, err := ai.ai.ChatStream(ctx, ai.initRequest(req))
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

				if data.Code != 0 {
					select {
					case <-ctx.Done():
						return
					case res <- Response{Error: data.Message, ErrorCode: fmt.Sprintf("ERR%d", data.Code)}:
					}
					return
				}

				var content string
				var finishReason string
				for _, c := range data.Data.Messages {
					content += c.Content
					finishReason = c.FinishReason
				}

				select {
				case <-ctx.Done():
					return
				case res <- Response{
					Text:         content,
					FinishReason: finishReason,
					InputTokens:  data.Usage.PromptTokens,
					OutputTokens: data.Usage.AnswerTokens,
				}:
				}
			}
		}
	}()

	return res, nil
}

func (ai *BaichuanAIChat) MaxContextLength(model string) int {
	// TODO 未找到相关文档记载
	return 4000
}
