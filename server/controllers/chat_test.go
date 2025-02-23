package controllers

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"github.com/mylxsw/go-utils/must"
	"github.com/mylxsw/go-utils/ternary"
	"testing"
)

func TestHandleChatResponse(t *testing.T) {
	// -------------- API Mode --------------
	simulateAPIMode(t, true, "Hello, world", "Great!", "Hello, world", "Great!")
	simulateAPIMode(t, true, "Hello, world", "", "Hello, world", "")
	simulateAPIMode(t, false, "Hello, world", "Great!", "Hello, world", "")
	simulateAPIMode(t, false, "Hello, world", "", "Hello, world", "")

	// -------------- Think Mode --------------
	simulateThinkMode(t, true, "<think></think>Hello, world", "Hello, world", "<think></think>")
	simulateThinkMode(t, true, "<think>Great!</think>Hello, world", "Hello, world", "<think>Great!</think>")
	simulateThinkMode(t, true, "Hello, world", "Hello, world", "")
	simulateThinkMode(t, true, "<think>hello, world", "<think>hello, world", "<think>hello, world")
	simulateThinkMode(t, true, "\n\n \n<think>thinking...</think>\nHello, world", "\n\n \n\nHello, world", "\n\n \n<think>thinking...</think>")

	simulateThinkMode(t, false, "<think></think>Hello, world", "Hello, world", "")
	simulateThinkMode(t, false, "<think>Great!</think>Hello, world", "Hello, world", "")
	simulateThinkMode(t, false, "Hello, world", "Hello, world", "")
	simulateThinkMode(t, false, "<think>hello, world", "<think>hello, world", "")
	simulateThinkMode(t, false, "\n\n \n<think>thinking...</think>\nHello, world", "\n\n \n\nHello, world", "")
}

func simulateAPIMode(t *testing.T, reasoning bool, message string, reason string, expectReply string, expectReasoning string) {

	fmt.Printf("--------------- API Mode --- reasoning: %v, message: %s ---------------\n", reasoning, message)

	req := &chat.Request{
		Model:    "gpt-3.5-turbo",
		Stream:   true,
		Flags:    ternary.If(reasoning, []string{"reasoning"}, []string{}),
		Messages: []chat.Message{},
	}

	stream := make(chan chat.Response)
	go func() {
		defer close(stream)

		{
			runes := []rune(reason)
			for i := 0; i < len(runes); i += 2 {
				end := i + 2
				if end > len(runes) {
					end = len(runes)
				}
				stream <- chat.Response{
					ReasoningContent: string(runes[i:end]),
				}
			}
		}

		{
			runes := []rune(message)
			for i := 0; i < len(runes); i += 2 {
				end := i + 2
				if end > len(runes) {
					end = len(runes)
				}
				stream <- chat.Response{
					Text: string(runes[i:end]),
				}
			}
		}
	}()

	actualReplyText := ""
	actualReasoningText := ""
	replyText, thinkingProcess, err := HandleChatResponse(context.TODO(), req, stream, &EventHandler{
		RequestContext: map[string]any{},
		WriteControlEvent: func(event FinalMessage) error {
			log.Debugf("control-event: %s", event.Type)
			return nil
		},
		WriteChatEvent: func(event ChatCompletionStreamResponse) error {
			if event.Type == "chat" {
				actualReplyText += event.Choices[0].Delta.Content
			} else if event.Type == "reasoning" {
				actualReasoningText += event.Choices[0].Delta.Content
			}

			log.Debugf("chat-event[%s]: %s", event.Type, event.Choices[0].Delta.Content)
			return nil
		},
	})
	must.NoError(err)

	log.WithFields(log.Fields{
		"expect_reply_text": replyText,
		"actual_reply_text": actualReplyText,
		"thinking_process":  thinkingProcess,
	}).Debug("chat response")

	assert.Equal(t, expectReply, actualReplyText)
	assert.Equal(t, expectReasoning, actualReasoningText)
}

func simulateThinkMode(t *testing.T, reasoning bool, message string, expectReply string, expectReasoning string) {

	fmt.Printf("--------------- Think Mode --- reasoning: %v, message: %s ---------------\n", reasoning, message)

	req := &chat.Request{
		Model:    "gpt-3.5-turbo",
		Stream:   true,
		Flags:    ternary.If(reasoning, []string{"reasoning"}, []string{}),
		Messages: []chat.Message{},
	}

	stream := make(chan chat.Response)
	go func() {
		defer close(stream)

		runes := []rune(message)
		for i := 0; i < len(runes); i += 2 {
			end := i + 2
			if end > len(runes) {
				end = len(runes)
			}
			stream <- chat.Response{
				Text: string(runes[i:end]),
			}
		}
	}()

	actualReplyText := ""
	actualReasoningText := ""
	replyText, thinkingProcess, err := HandleChatResponse(context.TODO(), req, stream, &EventHandler{
		RequestContext: map[string]any{},
		WriteControlEvent: func(event FinalMessage) error {
			log.Debugf("control-event: %s", event.Type)
			return nil
		},
		WriteChatEvent: func(event ChatCompletionStreamResponse) error {
			if event.Type == "chat" {
				actualReplyText += event.Choices[0].Delta.Content
			} else if event.Type == "reasoning" {
				actualReasoningText += event.Choices[0].Delta.Content
			}

			log.Debugf("chat-event[%s]: %s", event.Type, event.Choices[0].Delta.Content)
			return nil
		},
	})
	must.NoError(err)

	log.WithFields(log.Fields{
		"expect_reply_text": replyText,
		"actual_reply_text": actualReplyText,
		"thinking_process":  thinkingProcess,
	}).Debug("chat response")

	assert.Equal(t, expectReply, actualReplyText)
	assert.Equal(t, expectReasoning, actualReasoningText)
}

func TestSepThinkingContent(t *testing.T) {

	testSepThinkingContent(t, "<think></think>Hello", "Hello", "")
	testSepThinkingContent(t, "<think></think>\n\nHello", "\n\nHello", "")
	testSepThinkingContent(t, "\nHello", "\nHello", "")
	testSepThinkingContent(t, "Hello", "Hello", "")
	testSepThinkingContent(t, "\n<think></think>Hello", "\nHello", "")
	testSepThinkingContent(t, "<think>thinking...</think>\nHello", "\nHello", "thinking...")
	testSepThinkingContent(t, "\n\n<think>thinking...\n\n</think>\nHello\n", "\n\n\nHello\n", "thinking...\n\n")
}

func testSepThinkingContent(t *testing.T, data string, expectContent, expectThink string) {
	think, content := sepThinkingContent(data)
	fmt.Printf("think: %s, content: %s\n----\n", think, content)
	assert.Equal(t, expectContent, content)
	assert.Equal(t, expectThink, think)
}
