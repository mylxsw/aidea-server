package oneapi

import (
	"context"
	oai "github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/aidea-server/internal/misc"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
)

type OneAPI struct {
	client oai.Client
	trans  youdao.Translater
}

func New(client oai.Client, trans youdao.Translater) *OneAPI {
	return &OneAPI{client: client, trans: trans}
}

func (oa *OneAPI) ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan oai.ChatStreamResponse, error) {
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
