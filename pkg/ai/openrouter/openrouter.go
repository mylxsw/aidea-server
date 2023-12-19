package openrouter

import (
	"context"
	openai2 "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
	"strings"
)

type OpenRouter struct {
	client openai2.Client
}

func NewOpenRouter(client openai2.Client) *OpenRouter {
	return &OpenRouter{client: client}
}

func (oa *OpenRouter) ChatStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan openai2.ChatStreamResponse, error) {
	request.Model = strings.ReplaceAll(request.Model, ".", "/")
	return oa.client.ChatStream(ctx, request)
}

func (oa *OpenRouter) Chat(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	request.Model = strings.ReplaceAll(request.Model, ".", "/")
	return oa.client.CreateChatCompletion(ctx, request)
}

// 所有的模型在这里 https://openrouter.ai/docs#models
var supportModels = []string{
	"nousresearch/nous-capybara-34b",
	"nousresearch/nous-capybara-7b",
	"mistralai/mistral-7b-instruct",
	"huggingfaceh4/zephyr-7b-beta",
	"openchat/openchat-7b",
	"gryphe/mythomist-7b",
	"openrouter/cinematika-7b",
	"rwkv/rwkv-5-world-3b",
	"recursal/rwkv-5-3b-ai-town",
	"jebcarter/psyfighter-13b",
	"koboldai/psyfighter-13b-2",
	"nousresearch/nous-hermes-llama2-13b",
	"meta-llama/codellama-34b-instruct",
	"phind/phind-codellama-34b",
	"intel/neural-chat-7b",
	"mistralai/mixtral-8x7b-instruct",
	"haotian-liu/llava-13b",
	"nousresearch/nous-hermes-2-vision-7b",
	"meta-llama/llama-2-13b-chat",
	"openai/gpt-3.5-turbo",
	"openai/gpt-3.5-turbo-1106",
	"openai/gpt-3.5-turbo-0301",
	"openai/gpt-3.5-turbo-16k",
	"openai/gpt-4-1106-preview",
	"openai/gpt-4",
	"openai/gpt-4-0314",
	"openai/gpt-4-32k",
	"openai/gpt-4-32k-0314",
	"openai/gpt-4-vision-preview",
	"openai/text-davinci-002",
	"openai/gpt-3.5-turbo-instruct",
	"google/palm-2-chat-bison",
	"google/palm-2-codechat-bison",
	"google/palm-2-chat-bison-32k",
	"google/palm-2-codechat-bison-32k",
	"google/gemini-pro",
	"google/gemini-pro-vision",
	"perplexity/pplx-70b-online",
	"perplexity/pplx-7b-online",
	"perplexity/pplx-7b-chat",
	"perplexity/pplx-70b-chat",
	"meta-llama/llama-2-70b-chat",
	"nousresearch/nous-hermes-llama2-70b",
	"jondurbin/airoboros-l2-70b",
	"migtissera/synthia-70b",
	"teknium/openhermes-2-mistral-7b",
	"teknium/openhermes-2.5-mistral-7b",
	"pygmalionai/mythalion-13b",
	"undi95/remm-slerp-l2-13b",
	"xwin-lm/xwin-lm-70b",
	"gryphe/mythomax-l2-13b-8k",
	"undi95/toppy-m-7b",
	"alpindale/goliath-120b",
	"lizpreciatior/lzlv-70b-fp16-hf",
	"neversleep/noromaid-20b",
	"01-ai/yi-34b-chat",
	"01-ai/yi-34b",
	"01-ai/yi-6b",
	"togethercomputer/stripedhyena-nous-7b",
	"togethercomputer/stripedhyena-hessian-7b",
	"mistralai/mixtral-8x7b",
	"anthropic/claude-2",
	"anthropic/claude-2.0",
	"anthropic/claude-instant-v1",
	"anthropic/claude-v1",
	"anthropic/claude-1.2",
	"anthropic/claude-instant-v1-100k",
	"anthropic/claude-v1-100k",
	"anthropic/claude-instant-1.0",
	"mancer/weaver",
	"open-orca/mistral-7b-openorca",
	"gryphe/mythomax-l2-13b",
}

// SupportModel 判断是否支持某个模型
func SupportModel(model string) bool {
	return array.In(strings.ReplaceAll(model, ".", "/"), supportModels)
}
