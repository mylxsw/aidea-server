package chat

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/sky"
	"github.com/mylxsw/go-utils/array"
)

type SkyChat struct {
	ai *sky.Sky
}

func NewSkyChat(ai *sky.Sky) *SkyChat {
	return &SkyChat{ai: ai}
}

func (ai *SkyChat) initRequest(req Request) sky.Request {
	req.Messages = req.Messages.Fix()

	messages := array.Map(req.Messages, func(item Message, _ int) sky.Message {
		if item.Role == "assistant" {
			item.Role = "bot"
		}

		return sky.Message{
			Role:    item.Role,
			Content: item.Content,
		}
	})

	return sky.Request{
		Model:    req.Model,
		Messages: messages,
	}
}

func (ai *SkyChat) Chat(ctx context.Context, req Request) (*Response, error) {
	chatReq := ai.initRequest(req)
	resp, err := ai.ai.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("sky chat error: [%d] %s", resp.Code, resp.CodeMsg)
	}

	return &Response{
		Text: resp.RespData.Reply,
	}, nil
}

func (ai *SkyChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
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
					case res <- Response{Error: data.CodeMsg, ErrorCode: fmt.Sprintf("ERR%d", data.Code)}:
					}
					return
				}

				select {
				case <-ctx.Done():
					return
				case res <- Response{
					Text: data.RespData.Reply,
				}:
				}
			}
		}
	}()

	return res, nil
}

func (ai *SkyChat) MaxContextLength(model string) int {
	// TODO 未找到相关文档记载
	return 4000
}
