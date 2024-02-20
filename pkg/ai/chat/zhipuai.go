package chat

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/zhipuai"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"strings"
)

type ZhipuChat struct {
	ai *zhipuai.ZhipuAI
}

func NewZhipuChat(ai *zhipuai.ZhipuAI) *ZhipuChat {
	return &ZhipuChat{ai: ai}
}

func (ai *ZhipuChat) initRequest(req Request) zhipuai.ChatRequest {
	req.Model = strings.TrimPrefix(req.Model, "zhipu:")
	req.Messages = req.Messages.Fix()

	log.With(req.Messages).Debug("zhipu chat request")

	messages := array.Map(req.Messages, func(item Message, _ int) any {
		if req.Model == zhipuai.ModelGLM4V && len(item.MultipartContents) > 0 {
			return zhipuai.MultipartMessage{
				Role: item.Role,
				Content: array.Map(item.MultipartContents, func(m *MultipartContent, _ int) zhipuai.MultipartContent {
					res := zhipuai.MultipartContent{
						Type: m.Type,
						Text: m.Text,
					}
					if m.Type == "image_url" {
						if strings.HasPrefix(m.ImageURL.URL, "http://") || strings.HasPrefix(m.ImageURL.URL, "https://") {
							res.ImageURL = &zhipuai.MultipartContentImage{
								URL: m.ImageURL.URL,
							}
						} else {
							res.ImageURL = &zhipuai.MultipartContentImage{
								URL: misc.RemoveImageBase64Prefix(m.ImageURL.URL),
							}
						}

					}
					return res
				}),
			}
		}

		return zhipuai.Message{
			Role:    item.Role,
			Content: item.Content,
		}
	})

	return zhipuai.ChatRequest{
		Model:    req.Model,
		Messages: messages,
	}
}

func (ai *ZhipuChat) Chat(ctx context.Context, req Request) (*Response, error) {
	chatReq := ai.initRequest(req)
	resp, err := ai.ai.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil && resp.Error.Code != "" {
		return nil, fmt.Errorf("zhipuai chat error: [%s] %s", resp.Error.Code, resp.Error.Message)
	}

	return &Response{
		Text: resp.Choices[0].Message.Content,
	}, nil
}

func (ai *ZhipuChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
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

				if data.Error != nil && data.Error.Code != "" {
					select {
					case <-ctx.Done():
						return
					case res <- Response{Error: data.Error.Message, ErrorCode: fmt.Sprintf("ERR%s", data.Error.Code)}:
					}
					return
				}

				select {
				case <-ctx.Done():
					return
				case res <- Response{
					Text: data.Choices[0].Delta.Content,
				}:
				}
			}
		}
	}()

	return res, nil
}

func (ai *ZhipuChat) MaxContextLength(model string) int {
	if model == zhipuai.ModelGLM4 || model == zhipuai.ModelGLM3Turbo {
		return 120000
	}
	if model == zhipuai.ModelGLM4V {
		return 2000
	}

	return 4000
}
