package chat

import (
	"context"
	"encoding/base64"
	"github.com/mylxsw/aidea-server/pkg/ai/google"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/ternary"
	"strings"
)

type GoogleChat struct {
	gai *google.GoogleAI
}

func NewGoogleChat(gai *google.GoogleAI) *GoogleChat {
	return &GoogleChat{gai: gai}
}

func (chat *GoogleChat) initRequest(req Request) (*google.Request, error) {
	req.Messages = req.Messages.Fix()

	var systemMessages Messages
	var contextMessages Messages

	for _, msg := range req.Messages {
		m := Message{
			Role:              msg.Role,
			Content:           msg.Content,
			MultipartContents: msg.MultipartContents,
		}

		if msg.Role == "system" {
			systemMessages = append(systemMessages, m)
		} else {
			contextMessages = append(contextMessages, m)
		}
	}

	contextMessages = contextMessages.Fix()
	if len(systemMessages) > 0 {
		systemMessage := systemMessages[0]
		finalSystemMessages := make(Messages, 0)

		systemMessage.Role = "user"
		finalSystemMessages = append(
			finalSystemMessages,
			systemMessage,
			Message{
				Role:    "assistant",
				Content: "好的",
			},
		)

		contextMessages = append(finalSystemMessages, contextMessages...)
	}

	googleReq := google.Request{}

	googleReq.Contents = array.Map(contextMessages, func(msg Message, _ int) google.Message {
		contents := make([]google.MessagePart, 0)
		if len(msg.MultipartContents) == 0 {
			contents = append(contents, google.MessagePart{
				Text: msg.Content,
			})
		} else {
			for _, ct := range msg.MultipartContents {
				if ct.Text != "" {
					contents = append(contents, google.MessagePart{
						Text: ct.Text,
					})
				} else if ct.ImageURL != nil {
					if strings.HasPrefix(ct.ImageURL.URL, "http://") || strings.HasPrefix(ct.ImageURL.URL, "https://") {
						encoded, mimeType, err := uploader.DownloadRemoteFileAsBase64Raw(context.TODO(), ct.ImageURL.URL, true)
						if err == nil {
							contents = append(contents, google.MessagePart{
								InlineData: &google.MessagePartInlineData{
									MimeType: mimeType,
									Data:     encoded,
								},
							})
						} else {
							log.With(err).Errorf("download remote image failed: %s", ct.ImageURL.URL)
						}
					} else {
						data, mimeType, err := misc.DecodeBase64ImageWithMime(ct.ImageURL.URL)
						if err == nil {
							contents = append(contents, google.MessagePart{
								InlineData: &google.MessagePartInlineData{
									MimeType: mimeType,
									Data:     base64.StdEncoding.EncodeToString(data),
								},
							})
						}
					}
				}
			}
		}

		return google.Message{
			Role:  ternary.IfElse(msg.Role == "user", google.RoleUser, google.RoleModel),
			Parts: contents,
		}
	})

	return &googleReq, nil
}

func (chat *GoogleChat) Chat(ctx context.Context, req Request) (*Response, error) {
	googleReq, err := chat.initRequest(req)
	if err != nil {
		return nil, err
	}

	// Vision 模型必须要有图片才能用
	if req.Model == google.ModelGeminiProVision && !googleReq.HasImage() {
		return &Response{Text: "当前模型有以下限制，请您知晓：\n\n- 每次提问必须上传一张图片\n- 不支持多轮对话"}, nil
	}

	res, err := chat.gai.Chat(ctx, req.Model, *googleReq)
	if err != nil {
		return nil, err
	}

	resText := res.String()
	if req.Model == google.ModelGeminiProVision {
		resText += "\n\n> 注意：当前模型不支持多轮对话，对话结束"
	}

	return &Response{Text: resText}, nil
}

func (chat *GoogleChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	googleReq, err := chat.initRequest(req)
	if err != nil {
		return nil, err
	}

	// Vision 模型必须要有图片才能用
	if req.Model == google.ModelGeminiProVision && !googleReq.HasImage() {
		res := make(chan Response)
		go func() {
			defer close(res)
			select {
			case <-ctx.Done():
			case res <- Response{Text: "当前模型有以下限制，请您知晓：\n\n- 每次提问必须上传一张图片\n- 不支持多轮对话"}:
			}
		}()

		return res, nil
	}

	stream, err := chat.gai.ChatStream(ctx, req.Model, *googleReq)
	if err != nil {
		return nil, err
	}

	res := make(chan Response)
	go func() {
		defer func() {
			if req.Model == google.ModelGeminiProVision {
				select {
				case res <- Response{Text: "\n\n> 注意：当前模型不支持多轮对话，对话结束"}:
				}
			}

			close(res)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-stream:
				if !ok {
					return
				}

				if data.Error != nil && data.Error.Code != 0 {
					res <- Response{
						Error:     data.Error.Message,
						ErrorCode: data.Error.Status,
					}
					return
				}

				select {
				case <-ctx.Done():
				case res <- Response{Text: data.String()}:
				}
			}
		}
	}()

	return res, nil
}

func (chat *GoogleChat) MaxContextLength(model string) int {
	switch model {
	case google.ModelGeminiProVision: // 32K
		return 12000
	case google.ModelGeminiPro:
		return 30000
	}

	return 4000
}
