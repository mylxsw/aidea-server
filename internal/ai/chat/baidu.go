package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/mylxsw/aidea-server/internal/ai/baidu"
)

type BaiduAIChat struct {
	bai baidu.BaiduAI
}

func NewBaiduAIChat(bai baidu.BaiduAI) *BaiduAIChat {
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

	res := baidu.ChatRequest{}

	contextMessages = contextMessages.Fix()
	if len(systemMessages) > 0 {
		systemMessage := systemMessages[0]
		if baidu.SupportSystemMessage(baidu.Model(req.Model)) {
			res.System = systemMessage.Content
			if len(res.System) > 1024 {
				res.System = res.System[:1024]
			}
		} else {
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
	}

	res.Messages = contextMessages
	return res
}

func (chat *BaiduAIChat) Chat(ctx context.Context, req Request) (*Response, error) {
	res, err := chat.bai.Chat(ctx, baidu.Model(req.Model), chat.initRequest(req))
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

	stream, err := chat.bai.ChatStream(ctx, baidu.Model(req.Model), baiduReq)
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

				if data.ErrorCode != 0 {
					select {
					case <-ctx.Done():
					case res <- Response{Error: data.ErrorMessage, ErrorCode: fmt.Sprintf("ERR%d", data.ErrorCode)}:
					}
					return
				}

				select {
				case <-ctx.Done():
					return
				case res <- Response{
					Text:         data.Result,
					InputTokens:  data.Usage.PromptTokens,
					OutputTokens: data.Usage.TotalTokens - data.Usage.PromptTokens,
				}:
				}
			}
		}
	}()

	return res, nil
}

func (chat *BaiduAIChat) MaxContextLength(model string) int {
	switch baidu.Model(model) {
	case baidu.ModelErnieBot:
		// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/jlil56u11
		return 3000
	case baidu.ModelErnieBotTurbo:
		// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/4lilb2lpf
		return 7000
	case baidu.ModelLlama2_70b:
		// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/8lkjfhiyt
		return 3000
	case baidu.ModelLlama2_7b_CN:
		// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Sllyztytp
		return 3000
	case baidu.ModelChatGLM2_6B_32K:
		// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Bllz001ff
		return 3000
	case baidu.ModelAquilaChat7B:
		// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/ollz02e7i
		return 3000
	case baidu.ModelBloomz7B:
		// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Jljcadglj
		return 3000
	}

	return 3000
}
