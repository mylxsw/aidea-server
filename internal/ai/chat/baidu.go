package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/mylxsw/aidea-server/internal/ai/baidu"
)

type BaiduAIChat struct {
	bai *baidu.BaiduAI
}

func NewBaiduAIChat(bai *baidu.BaiduAI) *BaiduAIChat {
	return &BaiduAIChat{bai: bai}
}

func (chat *BaiduAIChat) initRequest(req Request) baidu.ChatRequest {
	req.Model = strings.TrimPrefix(req.Model, "文心千帆:")

	var systemMessages baidu.ChatMessages
	var contextMessages baidu.ChatMessages

	for _, msg := range req.Messages {
		m := baidu.ChatMessage{
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
		finalSystemMessages := make(baidu.ChatMessages, 0)

		systemMessage.Role = "user"
		finalSystemMessages = append(
			finalSystemMessages,
			systemMessage,
			baidu.ChatMessage{
				Role:    "assistant",
				Content: "好的",
			},
		)

		contextMessages = append(finalSystemMessages, contextMessages...)
	}

	return baidu.ChatRequest{Messages: contextMessages}
}

func (chat *BaiduAIChat) Chat(ctx context.Context, req Request) (*Response, error) {
	res, err := chat.bai.Chat(baidu.Model(req.Model), chat.initRequest(req))
	if err != nil {
		return nil, err
	}

	if res.ErrorCode != 0 {
		return nil, fmt.Errorf("baidu ai chat error: [%d] %s", res.ErrorCode, res.ErrorMessage)
	}

	return &Response{
		Text:         res.Result,
		InputTokens:  res.Usage.PromptTokens,
		OutputTokens: res.Usage.CompletionTokens,
	}, nil
}

func (chat *BaiduAIChat) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	baiduReq := chat.initRequest(req)
	baiduReq.Stream = true

	stream, err := chat.bai.ChatStream(baidu.Model(req.Model), baiduReq)
	if err != nil {
		return nil, err
	}

	res := make(chan Response)
	go func() {
		defer close(res)

		for data := range stream {
			if data.ErrorCode != 0 {
				res <- Response{
					Error:     data.ErrorMessage,
					ErrorCode: fmt.Sprintf("ERR%d", data.ErrorCode),
				}
				return
			}

			res <- Response{
				Text:         data.Result,
				InputTokens:  data.Usage.PromptTokens,
				OutputTokens: data.Usage.TotalTokens - data.Usage.PromptTokens,
			}
		}
	}()

	return res, nil
}
