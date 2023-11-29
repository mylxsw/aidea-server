package chat_test

import (
	chat2 "github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/go-utils/assert"
	"testing"
)

func TestReduceMessageContextUpToContextWindow(t *testing.T) {
	messages := chat2.Messages{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "你好啊，有什么需要帮助的"},
		{Role: "user", Content: "我想知道天气"},
		{Role: "assistant", Content: "你在哪个城市"},
		{Role: "user", Content: "北京"},
	}

	assert.EqualValues(t, 1, len(chat2.ReduceMessageContextUpToContextWindow(messages, 0)))
	assert.EqualValues(t, 3, len(chat2.ReduceMessageContextUpToContextWindow(messages, 1)))
	assert.EqualValues(t, 5, len(chat2.ReduceMessageContextUpToContextWindow(messages, 2)))
	assert.EqualValues(t, 5, len(chat2.ReduceMessageContextUpToContextWindow(messages, 3)))
}

func TestMessageTokenCount(t *testing.T) {
	messages := chat2.Messages{
		{Role: "user", Content: `OpenAI's large language models (sometimes referred to as GPT's) process text using tokens, which are common sequences of characters found in a set of text. The models learn to understand the statistical relationships between these tokens, and excel at producing the next token in a sequence of tokens.

You can use the tool below to understand how a piece of text might be tokenized by a language model, and the total count of tokens in that piece of text.

It's important to note that the exact tokenization process varies between models. Newer models like GPT-3.5 and GPT-4 use a different tokenizer than our legacy GPT-3 and Codex models, and will produce different tokens for the same input text.

OpenAI 的大型语言模型（有时称为 GPT）使用标记处理文本，标记是一组文本中常见的字符序列。 这些模型学习理解这些标记之间的统计关系，并擅长生成标记序列中的下一个标记。

您可以使用下面的工具来了解语言模型如何对一段文本进行标记，以及该文本中的标记总数。

值得注意的是，确切的标记化过程因模型而异。 较新的模型（例如 GPT-3.5 和 GPT-4）使用与旧版 GPT-3 和 Codex 模型不同的标记器，并且将为相同的输入文本生成不同的标记。`},
	}

	num, err := chat2.MessageTokenCount(messages, "gpt-4")
	assert.NoError(t, err)

	// https://platform.openai.com/tokenizer
	// OpenAI 官方的 Tokenizer 计算结果是 349，此处计算结果为 351，基本一致
	assert.True(t, num > 340 && num < 360)
}
