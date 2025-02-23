package controllers

import (
	"context"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/asteria/log"
	"github.com/sashabaranov/go-openai"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ChatCompletionStreamResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []ChatCompletionStreamChoice `json:"choices"`
	Type    string                       `json:"type,omitempty"`
}

type ChatCompletionStreamChoice struct {
	Index        int                             `json:"index"`
	Delta        ChatCompletionStreamChoiceDelta `json:"delta"`
	FinishReason *string                         `json:"finish_reason,omitempty"`
}

type ChatCompletionStreamChoiceDelta struct {
	Content      string               `json:"content"`
	Role         string               `json:"role,omitempty"`
	FunctionCall *openai.FunctionCall `json:"function_call,omitempty"`
}

type EventHandler struct {
	RequestContext    map[string]any
	WriteControlEvent func(event FinalMessage) error
	WriteChatEvent    func(event ChatCompletionStreamResponse) error
}

func HandleChatResponse(
	ctx context.Context,
	req *chat.Request,
	stream <-chan chat.Response,
	eventHandler *EventHandler,
) (replyText string, thinkingProcess ThinkingProcess, err error) {
	startTime := time.Now()

	// 发送 thinking 消息
	_ = eventHandler.WriteControlEvent(FinalMessage{Type: "thinking"})
	thinkingDone := sync.OnceFunc(func() {
		thinkingProcess.TimeConsumed = time.Since(startTime).Seconds()
		// 发送 thinking-done 消息
		_ = eventHandler.WriteControlEvent(FinalMessage{Type: "thinking-done", TimeConsumed: thinkingProcess.TimeConsumed})
	})

	// 深度推理模式：api|think|none
	var reasoningMode string

	chatBuffer := strings.Builder{}
	reasoningBuffer := strings.Builder{}

	reasoningWritten := ""
	chatWritten := ""

	defer func() {
		thinkingDone()
		if chatWritten == "" && thinkingProcess.Content != "" {
			_ = eventHandler.WriteChatEvent(buildChatCompletionStreamResponse(req.Model, 99999, "chat", thinkingProcess.Content))
		}
	}()

	// 生成 SSE 流
	timer := time.NewTimer(600 * time.Second)
	defer timer.Stop()

	id := 0
	for {
		if id > 0 {
			timer.Reset(60 * time.Second)
		}

		select {
		case <-timer.C:
			return replyText, thinkingProcess, ErrChatResponseGapTimeout
		case <-ctx.Done():
			return replyText, thinkingProcess, nil
		case res, ok := <-stream:
			if !ok {
				return replyText, thinkingProcess, nil
			}

			replyText += res.Text
			id++

			if res.ErrorCode != "" {
				chatBuffer.Reset()
				reasoningBuffer.Reset()

				if strings.TrimSpace(chatWritten) == "" && strings.TrimSpace(reasoningWritten) == "" {
					log.WithFields(eventHandler.RequestContext).Warningf("chat response failed, we need a retry: %v", res)
					return replyText, thinkingProcess, ErrChatResponseEmpty
				}

				log.WithFields(eventHandler.RequestContext).Errorf("chat response failed: %v", res)

				if res.Error != "" {
					replyText += fmt.Sprintf("\n\n---\nSorry, we encountered some errors. Here are the error details: \n```\n%s\n```\n", res.Error)
					_ = eventHandler.WriteChatEvent(buildChatCompletionStreamResponse(req.Model, id, "chat", replyText))
				}

				return replyText, thinkingProcess, nil
			}

			replyTextTrimmed := strings.TrimSpace(replyText)
			if reasoningMode == "" {
				if strings.TrimSpace(res.ReasoningContent) != "" {
					reasoningMode = "api"
				} else {
					if len(replyTextTrimmed) <= len("<think>") {
						chatBuffer.WriteString(res.Text)
						continue
					}

					if strings.HasPrefix(replyTextTrimmed, "<think>") {
						reasoningMode = "think"
						reasoningBuffer.WriteString(chatBuffer.String())
						chatBuffer.Reset()
					} else {
						reasoningMode = "none"
					}
				}
			}

			switch reasoningMode {
			case "api":
				if res.ReasoningContent != "" {
					reasoningBuffer.WriteString(res.ReasoningContent)
				}
				if res.Text != "" {
					chatBuffer.WriteString(res.Text)
				}
			case "think":
				if strings.HasPrefix(replyTextTrimmed, "<think>") {
					if strings.Contains(replyTextTrimmed, "</think>") {
						var thinkingContent string
						thinkingContent, replyText = sepThinkingContent(replyText)
						chatBuffer.WriteString(replyText)

						// 已经写出的 reasoning 内容为 <think>...
						// 判断 thinkingContent（完整 reasoning 内容） 与已经写出的之间的差异，将差异内容作为增量输出
						if strings.TrimSpace(reasoningWritten) != "" {
							incrementalReasoning := strings.TrimPrefix("<think>"+strings.TrimSpace(thinkingContent)+"</think>", strings.TrimSpace(reasoningWritten))
							reasoningBuffer.WriteString(incrementalReasoning)
						} else {
							incrementalReasoning := strings.TrimPrefix("<think>"+strings.TrimSpace(thinkingContent)+"</think>", strings.TrimSpace(thinkingProcess.Content))
							reasoningBuffer.WriteString(incrementalReasoning)
						}
					} else {
						reasoningBuffer.WriteString(res.Text)
					}
				} else {
					chatBuffer.WriteString(res.Text)
				}
			case "none":
				if res.Text != "" {
					chatBuffer.WriteString(res.Text)
				}
			default:
			}

			if reasoningBuffer.Len() > 0 {
				if req.EnableReasoning() {
					buffer := reasoningBuffer.String()
					thinkingProcess.Content += buffer
					reasoningWritten += buffer
					_ = eventHandler.WriteChatEvent(buildChatCompletionStreamResponse(req.Model, id, "reasoning", buffer))
					reasoningBuffer.Reset()
				} else {
					thinkingProcess.Content += strings.TrimPrefix(reasoningBuffer.String(), thinkingProcess.Content)
				}
			}

			if chatBuffer.Len() > 0 {
				thinkingDone()
				chatWritten += chatBuffer.String()
				_ = eventHandler.WriteChatEvent(buildChatCompletionStreamResponse(req.Model, id, "chat", chatBuffer.String()))
				chatBuffer.Reset()
			}
		}
	}
}

func buildChatCompletionStreamResponse(model string, id int, typ string, content string) ChatCompletionStreamResponse {
	return ChatCompletionStreamResponse{
		ID:      strconv.Itoa(id),
		Created: time.Now().Unix(),
		Model:   model,
		Object:  "chat.completion",
		Choices: []ChatCompletionStreamChoice{
			{
				Delta: ChatCompletionStreamChoiceDelta{
					Role:    "assistant",
					Content: content,
				},
			},
		},
		Type: typ,
	}
}

func sepThinkingContent(replyText string) (thinkingContent string, content string) {
	if strings.HasPrefix(strings.TrimSpace(replyText), "<think>") {
		start := strings.Index(replyText, "<think>")
		end := strings.Index(replyText, "</think>")
		if start != -1 && end != -1 && end > start {
			thinkingContent = replyText[start+len("<think>") : end]
			content = replyText[:start] + replyText[end+len("</think>"):]
			return
		}
	}

	return "", replyText
}
