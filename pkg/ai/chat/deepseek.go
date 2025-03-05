package chat

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/deepseek"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/uploader"
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
	if req.EnableReasoning() && !strings.Contains(req.GetSystemPrompt(), "<think>") {
		req = *req.MergeSystemPrompt("In every output, response using the following format:\n<think>\n{reasoning_content}\n</think>\n\n{content}")
	}

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
					Type: openai.ChatMessagePartType(item.Type),
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
						Detail: openai.ImageURLDetail(item.ImageURL.Detail),
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
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
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

				//log.With(data).Debugf("receive message from deepseek")

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
