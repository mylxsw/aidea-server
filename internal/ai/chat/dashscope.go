package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/go-utils/array"
)

type DashScopeChat struct {
	dashscope *dashscope.DashScope
}

func NewDashScopeChat(dashscope *dashscope.DashScope) *DashScopeChat {
	return &DashScopeChat{dashscope: dashscope}
}

func (ds *DashScopeChat) initRequest(req Request) dashscope.ChatRequest {
	req.Messages = req.Messages.Fix()

	var systemMessages Messages
	var contextMessages Messages

	for _, msg := range req.Messages {
		m := Message{
			Role:    msg.Role,
			Content: msg.Content,
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

	return dashscope.ChatRequest{
		Model: strings.TrimPrefix(req.Model, "灵积:"),
		Input: dashscope.ChatInput{
			Prompt:  req.Messages[len(req.Messages)-1].Content,
			History: array.Reverse(histories),
		},
		Parameters: dashscope.ChatParameters{
			EnableSearch: true,
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
	case dashscope.ModelQWenV1, dashscope.ModelQWenTurbo, dashscope.ModelQWenPlusV1, dashscope.ModelQWenPlus:
		// https://help.aliyun.com/zh/dashscope/developer-reference/api-details?disableWebsiteRedirect=true
		return 6000
	}

	return 4000
}
