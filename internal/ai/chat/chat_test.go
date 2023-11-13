package chat

import (
	"context"
	"testing"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

type ChatTestClient struct{}

func (c ChatTestClient) Chat(ctx context.Context, req Request) (*Response, error) {
	panic("implement me")
}

func (c ChatTestClient) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	panic("implement me")
}

func (c ChatTestClient) MaxContextLength(model string) int {
	return 2048
}

func TestRequestFix(t *testing.T) {
	req := Request{
		Messages: Messages{
			{Role: "system", Content: "system #1"},
			{Role: "user", Content: "user #1"},
			{Role: "assistant", Content: "assistant #1"},
			{Role: "user", Content: "user #2"},
			{Role: "assistant", Content: "assistant #2"},
			{Role: "user", Content: "user #3"},
		},
		Model: "gpt-3.5-turbo",
	}.Init()

	{
		fixed, _, err := req.Fix(ChatTestClient{}, 0)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(fixed.Messages))
	}

	{
		fixed, _, err := req.Fix(ChatTestClient{}, 1)
		assert.NoError(t, err)
		assert.Equal(t, 4, len(fixed.Messages))
	}

	{
		fixed, _, err := req.Fix(ChatTestClient{}, 2)
		assert.NoError(t, err)
		assert.Equal(t, 6, len(fixed.Messages))
	}
}

func TestMessages_Fix(t *testing.T) {
	messages := Messages{
		{Role: "system", Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃"},
		{Role: "user", Content: "老铁，最近怎么样？"},
		{Role: "user", Content: "怎么了？"},
		{Role: "user", Content: "你是谁？"},
		{Role: "user", Content: "你是谁？"},
		{Role: "assistant", Content: "我在呢，你有什么问题吗？"},
	}

	messages = messages.Fix()
	log.With(messages).Debug("messages")
}

func TestDashscopeChat_InitRequest(t *testing.T) {
	client := NewDashScopeChat(nil)
	{
		messages := Messages{
			{Role: "system", Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃"},
			{Role: "user", Content: "老铁，最近怎么样？"},
			{Role: "user", Content: "怎么了？"},
			{Role: "user", Content: "你是谁？"},
			{Role: "user", Content: "你是谁？"},
			{Role: "assistant", Content: "我在呢，你有什么问题吗？"},
		}

		res := client.initRequest(Request{Messages: messages})
		assert.Equal(t, "继续", res.Input.Prompt)
		assert.Equal(t, 2, len(res.Input.History))

		for i, msg := range res.Input.History {
			log.With(msg).Debugf("history #%d", i)
		}

		assert.Equal(t, 2, len(res.Input.History))

		log.Debug("=====================================")
	}

	{
		messages := Messages{
			{Role: "system", Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃"},
			{Role: "assistant", Content: "我在呢，你有什么问题吗？"},
			{Role: "user", Content: "老铁，最近怎么样？"},
		}

		res := client.initRequest(Request{Messages: messages})
		assert.Equal(t, "老铁，最近怎么样？", res.Input.Prompt)

		for i, msg := range res.Input.History {
			log.With(msg).Debugf("history #%d", i)
		}

		assert.Equal(t, 1, len(res.Input.History))

		log.Debug("=====================================")
	}

	{
		messages := Messages{
			{Role: "system", Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃"},
			{Role: "user", Content: "老铁，最近怎么样？"},
		}

		res := client.initRequest(Request{Messages: messages})
		assert.Equal(t, "老铁，最近怎么样？", res.Input.Prompt)

		for i, msg := range res.Input.History {
			log.With(msg).Debugf("history #%d", i)
		}

		assert.Equal(t, 1, len(res.Input.History))

		log.Debug("=====================================")
	}

	{
		messages := Messages{
			{Role: "user", Content: "老铁，最近怎么样？"},
		}

		res := client.initRequest(Request{Messages: messages})
		assert.Equal(t, "老铁，最近怎么样？", res.Input.Prompt)

		for i, msg := range res.Input.History {
			log.With(msg).Debugf("history #%d", i)
		}

		assert.Equal(t, 0, len(res.Input.History))

		log.Debug("=====================================")
	}

	{
		messages := Messages{
			{Role: "user", Content: "老铁，最近怎么样？"},
			{Role: "assistant", Content: "挺好的，你呢"},
			{Role: "user", Content: "我还挺好的，谢谢"},
		}

		res := client.initRequest(Request{Messages: messages})
		assert.Equal(t, "我还挺好的，谢谢", res.Input.Prompt)

		for i, msg := range res.Input.History {
			log.With(msg).Debugf("history #%d", i)
		}

		assert.Equal(t, 1, len(res.Input.History))

		log.Debug("=====================================")
	}

	{
		messages := Messages{
			{Role: "user", Content: "老铁，最近怎么样？"},
			{Role: "user", Content: "我还挺好的，谢谢"},
		}

		res := client.initRequest(Request{Messages: messages})
		assert.Equal(t, "我还挺好的，谢谢", res.Input.Prompt)

		for i, msg := range res.Input.History {
			log.With(msg).Debugf("history #%d", i)
		}

		assert.Equal(t, 0, len(res.Input.History))

		log.Debug("=====================================")
	}
}

func TestBaiduAIChat_InitRequest(t *testing.T) {
	client := NewBaiduAIChat(nil)

	{
		messages := Messages{
			{Role: "system", Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃"},
			{Role: "user", Content: "老铁，最近怎么样？"},
			{Role: "user", Content: "怎么了？"},
			{Role: "user", Content: "你是谁？"},
			{Role: "user", Content: "你是谁？"},
			{Role: "assistant", Content: "我在呢，你有什么问题吗？"},
		}

		res := client.initRequest(Request{Messages: messages})

		assert.Equal(t, 5, len(res.Messages))

		for i, msg := range res.Messages {
			log.With(msg).Debugf("history #%d", i)
		}

		log.Debug("=====================================")
	}

	{
		messages := Messages{
			{Role: "system", Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃"},
			{Role: "assistant", Content: "我在呢，你有什么问题吗？"},
		}

		res := client.initRequest(Request{Messages: messages})

		assert.Equal(t, 3, len(res.Messages))

		for i, msg := range res.Messages {
			log.With(msg).Debugf("history #%d", i)
		}

		log.Debug("=====================================")
	}

	{
		messages := Messages{
			{Role: "system", Content: "假如你是鲁迅，请使用批判性，略带讽刺的语言来回答我的问题，语言要风趣，幽默，略带调侃"},
			{Role: "assistant", Content: "我在呢，你有什么问题吗？"},
			{Role: "user", Content: "老铁，最近怎么样？"},
		}

		res := client.initRequest(Request{Messages: messages})

		assert.Equal(t, 3, len(res.Messages))

		for i, msg := range res.Messages {
			log.With(msg).Debugf("history #%d", i)
		}

		log.Debug("=====================================")
	}

	{
		messages := Messages{
			{Role: "assistant", Content: "我在呢，你有什么问题吗？"},
		}

		res := client.initRequest(Request{Messages: messages})

		assert.Equal(t, 1, len(res.Messages))

		for i, msg := range res.Messages {
			log.With(msg).Debugf("history #%d", i)
		}

		log.Debug("=====================================")
	}

	{
		messages := Messages{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "我在呢，你有什么问题吗？"},
			{Role: "assistant", Content: "我在呢，你有什么问题吗？"},
			{Role: "user", Content: "没啥，聊聊"},
		}

		res := client.initRequest(Request{Messages: messages})

		assert.Equal(t, 3, len(res.Messages))

		for i, msg := range res.Messages {
			log.With(msg).Debugf("history #%d", i)
		}

		log.Debug("=====================================")
	}

}
