package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"github.com/mylxsw/aidea-server/config"

	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/go-utils/array"
	"github.com/pkoukk/tiktoken-go"
	"github.com/sashabaranov/go-openai"
)

// SelectBestModel 根据字数选择最合适的模型
func SelectBestModel(model string, tokenCount int) string {
	if strings.HasPrefix(model, "gpt-3.5-turbo") && tokenCount <= 4000 {
		return "gpt-3.5-turbo-0613"
	}

	switch model {
	case "gpt-3.5-turbo":
		return "gpt-3.5-turbo-0613"
	case "gpt-3.5-turbo-16k":
		return "gpt-3.5-turbo-16k-0613"
		// case "gpt-4":
		// 	return "gpt-4-0613"
	}

	return model
}

// ModelMaxContextSize 模型最大上下文长度
func ModelMaxContextSize(model string) int {
	switch model {
	case "gpt-3.5-turbo": // fallthrough
	case "gpt-3.5-turbo-0613":
		return 4000
	case "gpt-3.5-turbo-16k": // fallthrough
	case "gpt-3.5-turbo-16k-0613":
		return 4000 * 4
	case "gpt-4":
		return 8000
	case "gpt-4-32k":
		return 4000 * 8
	}

	return 4000
}

// ReduceChatCompletionMessages 递归减少对话上下文
func ReduceChatCompletionMessages(messages []openai.ChatCompletionMessage, model string, maxTokens int) ([]openai.ChatCompletionMessage, int, error) {
	num, err := NumTokensFromMessages(messages, model)
	if err != nil {
		return nil, 0, fmt.Errorf("NumTokensFromMessages: %v", err)
	}

	if num <= maxTokens {
		return messages, num, nil
	}

	if len(messages) <= 1 {
		return nil, 0, errors.New("对话上下文过长，无法继续生成")
	}

	return ReduceChatCompletionMessages(messages[1:], model, maxTokens)
}

// WordCountForChatCompletionMessages 计算对话上下文的字数
func WordCountForChatCompletionMessages(messages []openai.ChatCompletionMessage) int64 {
	var count int64
	for _, msg := range messages {
		count += helper.WordCount(msg.Content)
	}

	return count
}

// NumTokensFromMessages 计算对话上下文的 token 数量
func NumTokensFromMessages(messages []openai.ChatCompletionMessage, model string) (numTokens int, err error) {
	// 所有非 gpt-3.5-turbo/gpt-4 的模型，都按照 gpt-3.5 的方式处理
	if !array.In(model, []string{"gpt-3.5-turbo", "gpt-4"}) {
		model = "gpt-3.5-turbo"
	}

	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return 0, fmt.Errorf("EncodingForModel: %v", err)
	}

	var tokensPerMessage int
	var tokensPerName int
	if strings.HasPrefix(model, "gpt-3.5-turbo") {
		tokensPerMessage = 4
		tokensPerName = -1
	} else if strings.HasPrefix(model, "gpt-4") {
		tokensPerMessage = 3
		tokensPerName = 1
	} else {
		tokensPerMessage = 3
		tokensPerName = 1
	}

	for _, message := range messages {
		numTokens += tokensPerMessage
		numTokens += len(tkm.Encode(message.Content, nil, nil))
		numTokens += len(tkm.Encode(message.Role, nil, nil))
		numTokens += len(tkm.Encode(message.Name, nil, nil))
		if message.Name != "" {
			numTokens += tokensPerName
		}
	}
	numTokens += 3
	return numTokens, nil
}

type OpenAI struct {
	conf    *config.Config
	clients []*openai.Client
}

func New(conf *config.Config, clients []*openai.Client) *OpenAI {
	return &OpenAI{clients: clients, conf: conf}
}

// client 随机返回一个 OpenAI Client
func (client *OpenAI) client(model string) *openai.Client {
	return client.clients[rand.Intn(len(client.clients))]
}

func (client *OpenAI) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	return client.client(request.Model).CreateChatCompletion(ctx, request)
}

func (client *OpenAI) CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (stream *openai.ChatCompletionStream, err error) {
	return client.client(request.Model).CreateChatCompletionStream(ctx, request)
}

type ChatStreamResponse struct {
	Code         string `json:"code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	ChatResponse *openai.ChatCompletionStreamResponse
}

func (client *OpenAI) ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan ChatStreamResponse, error) {
	stream, err := client.CreateChatCompletionStream(ctx, request)
	if err != nil {
		return nil, err
	}

	res := make(chan ChatStreamResponse)

	go func() {
		defer func() {
			close(res)
			stream.Close()
		}()

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}

			if err != nil {
				select {
				case <-ctx.Done():
				case res <- ChatStreamResponse{Code: "READ_STREAM_FAILED", ErrorMessage: fmt.Errorf("read stream failed: %v", err).Error()}:
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case res <- ChatStreamResponse{ChatResponse: &response}:
			}
		}
	}()

	return res, nil
}

func (client *OpenAI) CreateImage(ctx context.Context, request openai.ImageRequest) (response openai.ImageResponse, err error) {
	return client.client("dall-e").CreateImage(ctx, request)
}

func (client *OpenAI) CreateTranscription(ctx context.Context, request openai.AudioRequest) (response openai.AudioResponse, err error) {
	return client.client("audio").CreateTranscription(ctx, request)
}

func (client *OpenAI) QuickAsk(ctx context.Context, prompt string, question string, maxTokenCount int) (string, error) {
	if client.conf != nil && !client.conf.EnableOpenAI {
		return question, nil
	}

	var messages []openai.ChatCompletionMessage
	if prompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{Content: prompt, Role: openai.ChatMessageRoleSystem})
	}

	messages = append(messages, openai.ChatCompletionMessage{Content: question, Role: openai.ChatMessageRoleUser})

	req := openai.ChatCompletionRequest{
		Model:       SelectBestModel("gpt-3.5-turbo", 200),
		MaxTokens:   maxTokenCount,
		Temperature: 0.2,
		Messages:    messages,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	content := array.Reduce(
		resp.Choices,
		func(carry string, item openai.ChatCompletionChoice) string {
			return carry + "\n" + item.Message.Content
		},
		"",
	)

	return content, nil
}
