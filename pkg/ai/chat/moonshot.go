package chat

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/moonshot"
	oai "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
	"strings"
)

type MoonshotChat struct {
	oai *moonshot.Moonshot
}

func NewMoonshotChat(oai *moonshot.Moonshot) *MoonshotChat {
	return &MoonshotChat{oai: oai}
}

func (chat *MoonshotChat) initRequest(req Request) (*openai.ChatCompletionRequest, error) {
	req.Model = strings.TrimPrefix(req.Model, "moonshot:")

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

	msgs, tokenCount, err := oai.ReduceChatCompletionMessages(
		contextMessages,
		req.Model,
		oai.ModelMaxContextSize(req.Model),
	)
	if err != nil {
		return nil, err
	}

	messages := append(systemMessages, msgs...)
	req.Model = oai.SelectBestModel(req.Model, tokenCount)

	return &openai.ChatCompletionRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
	}, nil
}

func (chat *MoonshotChat) Chat(ctx context.Context, req Request) (*Response, error) {
	openaiReq, err := chat.initRequest(req)
	if err != nil {
		return nil, err
	}

	res, err := chat.oai.Chat(ctx, *openaiReq)
	if err != nil {
		if strings.Contains(err.Error(), "content management policy") {
			log.With(err).Errorf("违反 Azure OpenAI 内容管理策略")
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

func (chat *MoonshotChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
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
			}).Errorf("违反 Azure OpenAI 内容管理策略")
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
				}
			}
		}

	}()

	return res, nil
}

func (chat *MoonshotChat) MaxContextLength(model string) int {
	switch model {
	case moonshot.ModelMoonshotV1_8K: // 32K
		return 7000
	case moonshot.ModelMoonshotV1_32K:
		return 30000
	case moonshot.ModelMoonshotV1_128K:
		return 120000
	}

	return 7000
}
