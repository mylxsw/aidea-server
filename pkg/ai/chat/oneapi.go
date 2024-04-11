package chat

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/oneapi"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"strings"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
)

type OneAPIChat struct {
	oai *oneapi.OneAPI
}

func NewOneAPIChat(oai *oneapi.OneAPI) *OneAPIChat {
	return &OneAPIChat{oai: oai}
}

func (chat *OneAPIChat) initRequest(req Request) (*openai.ChatCompletionRequest, error) {
	req.Model = strings.TrimPrefix(req.Model, "oneapi:")

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
					} else {
						imageMimeType, err := misc.Base64ImageMediaType(url)
						if err == nil {
							url = misc.AddImageBase64Prefix(misc.RemoveImageBase64Prefix(url), imageMimeType)
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

	messages := append(systemMessages, contextMessages...)
	return &openai.ChatCompletionRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
	}, nil
}

func (chat *OneAPIChat) Chat(ctx context.Context, req Request) (*Response, error) {
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

func (chat *OneAPIChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
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

func (chat *OneAPIChat) MaxContextLength(model string) int {
	switch model {
	case "chatglm_turbo": // 32K
		return 32000
	}

	return 8000
}
