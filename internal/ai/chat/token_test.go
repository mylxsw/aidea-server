package chat_test

import (
	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/go-utils/assert"
	"testing"
)

func TestReduceMessageContextUpToContextWindow(t *testing.T) {
	messages := chat.Messages{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "你好啊，有什么需要帮助的"},
		{Role: "user", Content: "我想知道天气"},
		{Role: "assistant", Content: "你在哪个城市"},
		{Role: "user", Content: "北京"},
	}

	assert.EqualValues(t, 1, len(chat.ReduceMessageContextUpToContextWindow(messages, 0)))
	assert.EqualValues(t, 3, len(chat.ReduceMessageContextUpToContextWindow(messages, 1)))
	assert.EqualValues(t, 5, len(chat.ReduceMessageContextUpToContextWindow(messages, 2)))
	assert.EqualValues(t, 5, len(chat.ReduceMessageContextUpToContextWindow(messages, 3)))
}
