package chat

import (
	"context"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"strings"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
)

type OpenAIChat struct {
	oai openai2.Client
}

func NewOpenAIChat(oai openai2.Client) *OpenAIChat {
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

		if len(msg.MultipartContents) > 0 {
			m.Content = ""
			m.MultiContent = array.Map(msg.MultipartContents, func(item *MultipartContent, _ int) openai.ChatMessagePart {
				ret := openai.ChatMessagePart{
					Text: item.Text,
					Type: item.Type,
				}
				if item.Type == "image_url" && item.ImageURL != nil {
					url := item.ImageURL.URL
					if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
						encoded, err := uploader.DownloadRemoteFileAsBase64(context.TODO(), item.ImageURL.URL)
						if err == nil {
							url = encoded
						} else {
							log.With(err).Errorf("download remote image failed: %s", item.ImageURL.URL)
						}
					}

					ret.ImageURL = &openai.ChatMessageImageURL{
						URL:    url,
						Detail: item.ImageURL.Detail,
					}
				}

				return ret
			})
		}

		if msg.Role == "system" {
			systemMessages = append(systemMessages, m)
		} else {
			contextMessages = append(contextMessages, m)
		}
	}

	// 限制每次请求的最大字数
	//if (req.MaxTokens > 4096 || req.MaxTokens <= 0) && strings.HasPrefix(req.ArtisticType, "gpt-4") {
	//	req.MaxTokens = 1024
	//}

	msgs, tokenCount, err := openai2.ReduceChatCompletionMessages(
		contextMessages,
		req.Model,
		openai2.ModelMaxContextSize(req.Model),
	)
	if err != nil {
		return nil, err
	}

	messages := append(systemMessages, msgs...)
	req.Model = openai2.SelectBestModel(req.Model, tokenCount)

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

func (chat *OpenAIChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
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

func (chat *OpenAIChat) MaxContextLength(model string) int {
	return openai2.ModelMaxContextSize(model)
}
