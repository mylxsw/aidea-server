package oneapi

import (
	"context"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
)

type OneAPI struct {
	client openai2.Client
	trans  youdao.Translater
}

func New(client openai2.Client, trans youdao.Translater) *OneAPI {
	return &OneAPI{client: client, trans: trans}
}

func (oa *OneAPI) ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan openai2.ChatStreamResponse, error) {
	return oa.client.ChatStream(ctx, oa.translate(request))
}

func (oa *OneAPI) Chat(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	return oa.client.CreateChatCompletion(ctx, oa.translate(request))
}

func (oa *OneAPI) translate(request openai.ChatCompletionRequest) openai.ChatCompletionRequest {
	// Google PaLM-2 模型不支持中文，需要翻译为英文
	if oa.trans != nil && request.Model == "PaLM-2" {
		request.Messages = array.Map(request.Messages, func(item openai.ChatCompletionMessage, _ int) openai.ChatCompletionMessage {
			if !misc.ContainChinese(item.Content) {
				return item
			}

			item.Content = oa.trans.TranslateToEnglish(item.Content)
			return item
		})
	}

	log.With(request).Debugf("request")

	return request
}
