package chat

import (
	"context"
	"strings"

	oai "github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
)

type OpenAIChat struct {
	oai *oai.OpenAI
}

func NewOpenAIChat(oai *oai.OpenAI) *OpenAIChat {
	return &OpenAIChat{oai: oai}
}

func (chat *OpenAIChat) initRequest(req Request) (*openai.ChatCompletionRequest, error) {
	req.Model = strings.TrimPrefix(req.Model, "openai:")

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

	// 限制每次请求的最大字数
	//if (req.MaxTokens > 4096 || req.MaxTokens <= 0) && strings.HasPrefix(req.Model, "gpt-4") {
	//	req.MaxTokens = 1024
	//}

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

func (chat *OpenAIChat) Chat(ctx context.Context, req Request) (*Response, error) {
	openaiReq, err := chat.initRequest(req)
	if err != nil {
		return nil, err
	}

	res, err := chat.oai.CreateChatCompletion(ctx, *openaiReq)
	if err != nil {
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

func (chat *OpenAIChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	openaiReq, err := chat.initRequest(req)
	if err != nil {
		return nil, err
	}

	openaiReq.Stream = true

	stream, err := chat.oai.ChatStream(ctx, *openaiReq)
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

func (chat *OpenAIChat) MaxContextLength(model string) int {
	return oai.ModelMaxContextSize(model)
}
