package chat

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/dashscope"
	"github.com/mylxsw/aidea-server/pkg/file"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/str"
	"strings"
	"time"

	"github.com/mylxsw/go-utils/array"
)

type DashScopeChat struct {
	dashscope *dashscope.DashScope
	file      *file.File
}

func NewDashScopeChat(dashscope *dashscope.DashScope, file *file.File) *DashScopeChat {
	return &DashScopeChat{dashscope: dashscope, file: file}
}

func (ds *DashScopeChat) initRequest(req Request) dashscope.ChatRequest {
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

	input := dashscope.ChatInput{}

	if req.Model == dashscope.ModelQWenVLPlus || req.Model == dashscope.ModelQWenVLMax || strings.Contains(req.Model, "-vl-") {
		input.Messages = array.Map(contextMessages, func(msg Message, _ int) dashscope.Message {
			contents := make([]dashscope.MessageContent, 0)
			if len(msg.MultipartContents) == 0 {
				contents = append(contents, dashscope.MessageContent{
					Text: msg.Content,
				})
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				for _, ct := range msg.MultipartContents {
					if ct.Text != "" {
						contents = append(contents, dashscope.MessageContent{
							Text: ct.Text,
						})
					} else if ct.ImageURL != nil {
						imageURL := ct.ImageURL.URL
						if strings.HasPrefix(imageURL, "data:") {
							// 替换为图片 URL
							imageData, imageExt, err := misc.DecodeBase64Image(imageURL)
							if err == nil {
								res, err := ds.file.UploadTempFileData(ctx, imageData, strings.TrimPrefix(imageExt, "."), 7)
								if err != nil {
									log.Errorf("upload temp file failed: %v", err)
								} else {
									imageURL = res
								}
							} else {
								log.Errorf("decode base64 image failed: %v", err)
							}
						}

						if !strings.HasPrefix(imageURL, "data:") {
							contents = append(contents, dashscope.MessageContent{
								Image: imageURL,
							})
						}
					}
				}
			}

			return dashscope.Message{
				Role:    msg.Role,
				Content: contents,
			}
		})
	} else {
		histories := make([]dashscope.ChatHistory, 0)
		history := dashscope.ChatHistory{}
		for i, msg := range array.Reverse(contextMessages[:len(contextMessages)-1]) {
			if msg.Role == "user" {
				history.User = msg.Content
			} else {
				history.Bot = msg.Content
			}

			if i%2 == 1 {
				histories = append(histories, history)
				history = dashscope.ChatHistory{}
			}
		}

		input.Prompt = contextMessages[len(req.Messages)-1].Content
		input.History = array.Reverse(histories)
	}

	// 并不是所有模型都支持搜索，目前没有找到文档记载
	enableSearch := str.In(req.Model, []string{dashscope.ModelQWenPlus, dashscope.ModelQWenMax, dashscope.ModelQWenMaxLongContext})

	return dashscope.ChatRequest{
		Model: strings.TrimPrefix(req.Model, "灵积:"),
		Input: input,
		Parameters: dashscope.ChatParameters{
			EnableSearch: enableSearch,
		},
	}
}

func (ds *DashScopeChat) Chat(ctx context.Context, req Request) (*Response, error) {
	chatReq := ds.initRequest(req)
	resp, err := ds.dashscope.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	if resp.Code != "" {
		return nil, fmt.Errorf("dashscope chat error: [%s] %s", resp.Code, resp.Message)
	}

	return &Response{
		Text:         resp.Output.Text,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}, nil
}

func (ds *DashScopeChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	stream, err := ds.dashscope.ChatStream(ctx, ds.initRequest(req))
	if err != nil {
		return nil, err
	}

	res := make(chan Response)
	go func() {
		defer close(res)

		var lastMessage string
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-stream:
				if !ok {
					return
				}

				if data.Code != "" {
					select {
					case <-ctx.Done():
					case res <- Response{Error: data.Message, ErrorCode: data.Code}:
					}
					return
				}

				select {
				case <-ctx.Done():
					return
				case res <- Response{
					Text:         strings.TrimPrefix(data.Output.Text, lastMessage),
					InputTokens:  data.Usage.InputTokens,
					OutputTokens: data.Usage.OutputTokens,
				}:
				}

				lastMessage = data.Output.Text
			}
		}
	}()

	return res, nil
}

func (ds *DashScopeChat) MaxContextLength(model string) int {
	switch model {
	case dashscope.ModelQWenV1, dashscope.ModelQWenTurbo, dashscope.ModelQWenVLPlus,
		dashscope.ModelQWenPlusV1, dashscope.ModelQWenPlus, dashscope.ModelQWenMax,
		dashscope.ModelQWen7BChat, dashscope.ModelQWen14BChat:
		// https://help.aliyun.com/zh/dashscope/developer-reference/api-details?disableWebsiteRedirect=true
		return 6000
	case dashscope.ModelQWenMaxLongContext:
		// https://help.aliyun.com/zh/dashscope/developer-reference/api-details?spm=a2c4g.11186623.0.0.1a8e6ffdMzDGXm
		return 25000
	}

	return 4000
}
