package chat

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/deepseek"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
	"strings"
)

type DeepSeekChat struct {
	oai *deepseek.DeepSeek
}

func NewDeepSeekChat(oai *deepseek.DeepSeek) *DeepSeekChat {
	return &DeepSeekChat{oai: oai}
}

func (chat *DeepSeekChat) initRequest(req Request) (*openai.ChatCompletionRequest, error) {
	req.Model = strings.TrimPrefix(req.Model, "deepseek:")

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

	messages := append(systemMessages, contextMessages...)
	return &openai.ChatCompletionRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
	}, nil
}

func (chat *DeepSeekChat) Chat(ctx context.Context, req Request) (*Response, error) {
	openaiReq, err := chat.initRequest(req)
	if err != nil {
		return nil, err
	}

	res, err := chat.oai.Chat(ctx, *openaiReq)
	if err != nil {
		if strings.Contains(err.Error(), "content management policy") {
			log.With(err).Errorf("Violation of OpenAI content management policy")
			return nil, ErrContentFilter
		}

		return nil, err
	}

	return &Response{
		Text: array.Reduce(
			res.Choices,
			func(carry string, item openai.ChatCompletionChoice) string {
				return carry + "\n" + item.Message.Content
			},
			"",
		),
		InputTokens:  res.Usage.PromptTokens,
		OutputTokens: res.Usage.CompletionTokens,
	}, nil
}

func (chat *DeepSeekChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	openaiReq, err := chat.initRequest(req)
	if err != nil {
		return nil, err
	}

	openaiReq.Stream = true

	stream, err := chat.oai.ChatStream(ctx, *openaiReq)
	if err != nil {
		if strings.Contains(err.Error(), "content management policy") {
			log.WithFields(log.Fields{
				"error":   err,
				"message": req.assembleMessage(),
				"model":   req.Model,
				"room_id": req.RoomID,
			}).Errorf("Violation of OpenAI content management policy")
			return nil, ErrContentFilter
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

				if data.Code != "" {
					res <- Response{
						Error:     data.ErrorMessage,
						ErrorCode: data.Code,
					}
					return
				}

				res <- Response{
					Text: array.Reduce(
						data.ChatResponse.Choices,
						func(carry string, item openai.ChatCompletionStreamChoice) string {
							return carry + item.Delta.Content
						},
						"",
					),
					// DeepSeek 深度推理过程
					ReasoningContent: array.Reduce(
						data.ChatResponse.Choices,
						func(carry string, item openai.ChatCompletionStreamChoice) string {
							return carry + item.Delta.ReasoningContent
						},
						"",
					),
				}
			}
		}

	}()

	return res, nil
}

func (chat *DeepSeekChat) MaxContextLength(model string) int {
	return 4000
}
