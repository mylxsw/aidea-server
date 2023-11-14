package chat

import (
	"errors"
	"fmt"
	"github.com/mylxsw/go-utils/array"
	"github.com/pkoukk/tiktoken-go"
	"strings"
)

// ReduceMessageContextUpToContextWindow 减少对话上下文到指定的上下文窗口大小
func ReduceMessageContextUpToContextWindow(messages Messages, maxContext int) Messages {
	// q+a q+a ... q+a q
	// max = 2 , total = 3 => total[total - max:]
	if len(messages)-1 > maxContext*2 {
		messages = messages[len(messages)-maxContext*2-1:]
	}

	return messages
}

// ReduceMessageContext 递归减少对话上下文
func ReduceMessageContext(messages Messages, model string, maxTokens int) (reducedMessages Messages, tokenCount int, err error) {
	num, err := MessageTokenCount(messages, model)
	if err != nil {
		return nil, 0, fmt.Errorf("MessageTokenCount: %v", err)
	}

	if num <= maxTokens {
		// 第一个消息应该是 user 消息
		if len(messages) > 1 && messages[0].Role == "assistant" {
			return messages[1:], num, nil
		}

		return messages, num, nil
	}

	if len(messages) <= 1 {
		return nil, 0, errors.New("对话上下文过长，无法继续生成")
	}

	return ReduceMessageContext(messages[1:], model, maxTokens)
}

// MessageTokenCount 计算对话上下文的 token 数量
// TODO 不通厂商模型的 Token 计算方式可能不同，需要根据厂商模型进行区分
func MessageTokenCount(messages Messages, model string) (numTokens int, err error) {
	// 所有非 gpt-3.5-turbo/gpt-4 的模型，都按照 gpt-3.5 的方式处理
	if !array.In(model, []string{"gpt-3.5-turbo", "gpt-4"}) {
		model = "gpt-3.5-turbo"
	}

	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return 0, fmt.Errorf("EncodingForModel: %v", err)
	}

	var tokensPerMessage int
	if strings.HasPrefix(model, "gpt-3.5-turbo") {
		tokensPerMessage = 4
	} else if strings.HasPrefix(model, "gpt-4") {
		tokensPerMessage = 3
	} else {
		tokensPerMessage = 3
	}

	for _, message := range messages {
		numTokens += tokensPerMessage
		numTokens += len(tkm.Encode(message.Content, nil, nil))
		numTokens += len(tkm.Encode(message.Role, nil, nil))
	}
	numTokens += 3
	return numTokens, nil
}
