package openai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/go-utils/must"
	"io"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	openailib "github.com/sashabaranov/go-openai"
)

func TestTokenCal(t *testing.T) {
	messages := []openailib.ChatCompletionMessage{
		{Role: "system", Content: "你是一名 AI 助手，能够帮助用户打工"},
		{Role: "user", Content: "你好，我想要一份简历"},
		{Role: "assistant", Content: "好的，你的简历已经生成，可以通过以下链接下载："},
		{Role: "assistant", Content: "https://ai.mylxsw.com/resume/123456"},
		{Role: "user", Content: "谢谢"},
		{Role: "assistant", Content: "Do you have plans to add this functionality to the current SDK. I would love to contribute, but my level is far from enough, sorry."},
	}

	num, err := openai.NumTokensFromMessages(messages, "gpt-3.5-turbo")
	assert.NoError(t, err)

	fmt.Println(num)
	fmt.Println(openai.WordCountForChatCompletionMessages(messages))

	res, cnt, err := openai.ReduceChatCompletionMessages(messages, "gpt-3.5-turbo", 100)
	assert.NoError(t, err)
	fmt.Println(res)
	fmt.Println(cnt)
}

var prompt = `As an artistic assistant, your task is to create detailed prompts for Stable Diffusion to generate high-quality images based on themes I'll provide. 

## Prompt Concept
- Prompts comprise of a "Prompt:" and "Negative Prompt:" section, filled with tags separated by commas.
- Tags describe image content or elements to exclude in the generated image.

## () and [] Syntax
Brackets adjust keyword strength. (keyword) increases strength by 1.1 times while [keyword] reduces it by 0.9 times.

## Prompt Format Requirements
Prompts should detail people, scenery, objects, or abstract digital artworks and include at least five visual details.

### 1. Prompt Requirements
- Describe the main subject, texture, additional details, image quality, art style, color tone, and lighting. Avoid segmented descriptions, ":" or ".".
- For themes related to people, describe the eyes, nose, and lips to avoid deformation. Also detail appearance, emotions, clothing, posture, perspective, actions, background, etc.
- Texture refers to the artwork material.
- Image quality should start with "(best quality, 4k, 8k, highres, masterpiece:1.2), ultra-detailed, (realistic, photorealistic, photo-realistic:1.37),".
- Include the art style and control the image's overall color.
- Describe the image's lighting.

### 2. Negative Prompt Requirements
- Exclude: "nsfw, (low quality, normal quality, worst quality, jpeg artifacts), cropped, monochrome, lowres, low saturation, ((watermark)), (white letters)".
- For themes related to people, also exclude: "skin spots, acnes, skin blemishes, age spots, mutated hands, mutated fingers, deformed, bad anatomy, disfigured, poorly drawn face, extra limb, ugly, poorly drawn hands, missing limb, floating limbs, disconnected limbs, out of focus, long neck, long body, extra fingers, fewer fingers, (multi nipples), bad hands, signature, username, bad feet, blurry, bad body".

### 3. Limitations:
- Tags should be English words or phrases, not necessarily provided by me, with no sentences or explanations.
- Keep tag count within 40 and word count within 60.
- Exclude quotation marks("") in tags, and separate tags by commas.
- Arrange tags in order of importance.
- Themes may be in Chinese, but your output must be in English. 

Output as a json, with 'prompt' and 'negative_prompt' as keys.`

type Parameter struct {
	Description string `json:"description"`
	Type        string `json:"type"`
}

type Properties map[string]Parameter

func TestPromptFunctionRequest(t *testing.T) {
	openaiConf := openailib.DefaultConfig(os.Getenv("OPENAI_API_KEY"))
	openaiConf.HTTPClient.Timeout = 300 * time.Second
	openaiConf.APIType = openailib.APITypeOpenAI

	client := openailib.NewClientWithConfig(openaiConf)

	resp, err := client.CreateChatCompletion(context.TODO(), openailib.ChatCompletionRequest{
		MaxTokens: 500,
		Model:     "gpt-3.5-turbo",
		Messages: []openailib.ChatCompletionMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: "奔跑的蜗牛"},
		},
		Temperature: 0.2,
		User:        "user",
	})
	assert.NoError(t, err)

	log.With(resp).Debugf("response")

	for _, choice := range resp.Choices {
		if choice.FinishReason == "function_call" {
			switch choice.Message.FunctionCall.Name {
			case "prompt_validation":
				var arg PromptArg
				assert.NoError(t, json.Unmarshal([]byte(choice.Message.Content), &arg))

				arg.Prompt = regexp.MustCompile(`[\w\s]+:`).ReplaceAllString(arg.Prompt, "")

				log.With(arg).Debugf("prompt validation")
			}
		} else {
			var arg PromptArg
			assert.NoError(t, json.Unmarshal([]byte(choice.Message.Content), &arg))

			arg.Prompt = regexp.MustCompile(`[\w\s]+:`).ReplaceAllString(arg.Prompt, "")
			arg.NegativePrompt1 = regexp.MustCompile(`[\w\s]+:`).ReplaceAllString(arg.NegativePrompt1, "")
			arg.NegativePrompt2 = regexp.MustCompile(`[\w\s]+:`).ReplaceAllString(arg.NegativePrompt2, "")

			log.With(arg).Debugf("prompt validation")
		}

	}
}

type PromptArg struct {
	Prompt          string `json:"prompt,omitempty"`
	NegativePrompt1 string `json:"negativePrompt,omitempty"`
	NegativePrompt2 string `json:"negative_prompt,omitempty"`
}

func TestOpenAI_CreateSpeech(t *testing.T) {
	openaiConf := openailib.DefaultConfig(os.Getenv("OPENAI_API_KEY"))
	openaiConf.HTTPClient.Timeout = 300 * time.Second
	openaiConf.APIType = openailib.APITypeOpenAI

	client := openailib.NewClientWithConfig(openaiConf)

	speech := must.Must(client.CreateSpeech(context.TODO(), openailib.CreateSpeechRequest{
		Model: "tts-1",
		Input: "你好，我是一名 AI 助手，我能够帮助你打工",
		Voice: "nova",
	}))

	os.WriteFile("/tmp/speech.mp3", must.Must(io.ReadAll(speech)), 0644)
}
