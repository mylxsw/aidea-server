package openai

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/ai/control"
	"github.com/mylxsw/asteria/log"
	"github.com/sashabaranov/go-openai"
	"io"
)

type Client interface {
	CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error)
	CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (stream *openai.ChatCompletionStream, err error)
	ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan ChatStreamResponse, error)
	CreateImage(ctx context.Context, request openai.ImageRequest) (response openai.ImageResponse, err error)
	CreateTranscription(ctx context.Context, request openai.AudioRequest) (response openai.AudioResponse, err error)
	CreateSpeech(ctx context.Context, request openai.CreateSpeechRequest) (response io.ReadCloser, err error)
	QuickAsk(ctx context.Context, prompt string, question string, maxTokenCount int) (string, error)
}

type ClientImpl struct {
	main   Client
	backup Client
}

func (proxy *ClientImpl) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	ctl := control.FromContext(ctx)
	if ctl.PreferBackup && proxy.backup != nil {
		return proxy.backup.CreateChatCompletion(ctx, request)
	}

	if proxy.main != nil {
		response, err = proxy.main.CreateChatCompletion(ctx, request)
		if err == nil {
			return response, nil
		}
	}

	if proxy.backup != nil {
		log.WithFields(log.Fields{
			"request": request,
			"error":   err.Error(),
		}).Warningf("use control openai client")
		return proxy.backup.CreateChatCompletion(ctx, request)
	}

	return response, err
}

func (proxy *ClientImpl) CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (stream *openai.ChatCompletionStream, err error) {
	ctl := control.FromContext(ctx)
	if ctl.PreferBackup && proxy.backup != nil {
		return proxy.backup.CreateChatCompletionStream(ctx, request)
	}

	if proxy.main != nil {
		stream, err = proxy.main.CreateChatCompletionStream(ctx, request)
		if err == nil {
			return stream, nil
		}
	}

	if proxy.backup != nil {
		log.WithFields(log.Fields{
			"request": request,
			"error":   err.Error(),
		}).Warningf("use control openai client")
		return proxy.backup.CreateChatCompletionStream(ctx, request)
	}

	return stream, err
}

func (proxy *ClientImpl) ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan ChatStreamResponse, error) {
	ctl := control.FromContext(ctx)
	if ctl.PreferBackup && proxy.backup != nil {
		return proxy.backup.ChatStream(ctx, request)
	}

	var stream <-chan ChatStreamResponse
	var err error

	if proxy.main != nil {
		stream, err = proxy.main.ChatStream(ctx, request)
		if err == nil {
			return stream, nil
		}
	}

	if proxy.backup != nil {
		log.WithFields(log.Fields{
			"request": request,
			"error":   err.Error(),
		}).Warningf("use control openai client")
		return proxy.backup.ChatStream(ctx, request)
	}

	return stream, err
}

func (proxy *ClientImpl) CreateImage(ctx context.Context, request openai.ImageRequest) (response openai.ImageResponse, err error) {
	ctl := control.FromContext(ctx)
	if ctl.PreferBackup && proxy.backup != nil {
		return proxy.backup.CreateImage(ctx, request)
	}

	if proxy.main != nil {
		return proxy.main.CreateImage(ctx, request)
	}

	if proxy.backup != nil {
		return proxy.backup.CreateImage(ctx, request)
	}

	panic("no openai client available")
}

func (proxy *ClientImpl) CreateTranscription(ctx context.Context, request openai.AudioRequest) (response openai.AudioResponse, err error) {
	ctl := control.FromContext(ctx)
	if ctl.PreferBackup && proxy.backup != nil {
		return proxy.backup.CreateTranscription(ctx, request)
	}

	if proxy.main != nil {
		return proxy.main.CreateTranscription(ctx, request)
	}

	if proxy.backup != nil {
		return proxy.backup.CreateTranscription(ctx, request)
	}

	panic("no openai client available")
}

func (proxy *ClientImpl) CreateSpeech(ctx context.Context, request openai.CreateSpeechRequest) (response io.ReadCloser, err error) {
	ctl := control.FromContext(ctx)
	if ctl.PreferBackup && proxy.backup != nil {
		return proxy.backup.CreateSpeech(ctx, request)
	}

	if proxy.main != nil {
		return proxy.main.CreateSpeech(ctx, request)
	}

	if proxy.backup != nil {
		return proxy.backup.CreateSpeech(ctx, request)
	}

	panic("no openai client available")
}

func (proxy *ClientImpl) QuickAsk(ctx context.Context, prompt string, question string, maxTokenCount int) (string, error) {
	var res string
	var err error
	if proxy.main != nil {
		res, err = proxy.main.QuickAsk(ctx, prompt, question, maxTokenCount)
		if err == nil {
			return res, nil
		}
	}

	if proxy.backup != nil {
		log.WithFields(log.Fields{
			"prompt":          prompt,
			"question":        question,
			"max_token_count": maxTokenCount,
			"error":           err.Error(),
		}).Error("use control openai client")
		return proxy.backup.QuickAsk(ctx, prompt, question, maxTokenCount)
	}

	return res, err
}

func NewOpenAIProxy(main Client, backup Client) Client {
	return &ClientImpl{main: main, backup: backup}
}
