package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Anthropic struct {
	apiKey    string
	serverURL string

	client *http.Client
}

type Model string

const (
	ModelClaudeInstant Model = "claude-instant-1"
	ModelClaude2       Model = "claude-2"
)

func New(serverURL, apiKey string, client *http.Client) *Anthropic {
	if serverURL == "" {
		serverURL = "https://api.anthropic.com"
	}

	if client == nil {
		client = http.DefaultClient
	}

	return &Anthropic{apiKey: apiKey, serverURL: serverURL, client: client}
}

func (ai *Anthropic) Chat(ctx context.Context, req Request) (*Response, error) {
	req.Stream = false
	if req.MaxTokensToSample <= 0 {
		req.MaxTokensToSample = 4000
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %s", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(ai.serverURL, "/")+"/v1/complete", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request failed: %s", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("x-api-key", ai.apiKey)

	httpResp, err := ai.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chat failed: %s", err)
	}

	defer httpResp.Body.Close()

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("chat failed [%s]: %s", httpResp.Status, string(data))
	}

	var chatResp Response
	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response failed: %s", err)
	}

	return &chatResp, nil
}

func (ai *Anthropic) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	req.Stream = true
	if req.MaxTokensToSample <= 0 {
		req.MaxTokensToSample = 4000
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(ai.serverURL, "/")+"/v1/complete", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("x-api-key", ai.apiKey)

	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	httpResp, err := ai.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()

		return nil, fmt.Errorf("chat failed [%s]: %s", httpResp.Status, string(data))
	}

	res := make(chan Response)
	go func() {
		defer func() {
			_ = httpResp.Body.Close()
			close(res)
		}()

		reader := bufio.NewReader(httpResp.Body)
		for {
			data, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return
				}

				select {
				case <-ctx.Done():
				case res <- Response{Error: &ResponseError{Type: "read_error", Message: fmt.Sprintf("read response failed: %v", err)}}:
				}
				return
			}

			dataStr := strings.TrimSpace(string(data))
			if dataStr == "" {
				continue
			}

			if !strings.HasPrefix(dataStr, "data: ") {
				// event: completion
				// data: {"completion": "!", "stop_reason": null, "model": "claude-2.0"}
				//
				// event: ping
				// data: {}
				//
				// event: completion
				// data: {"completion": "", "stop_reason": "stop_sequence", "model": "claude-2.0"}
				continue
			}

			var chatResponse Response
			if err := json.Unmarshal([]byte(dataStr[6:]), &chatResponse); err != nil {
				select {
				case <-ctx.Done():
				case res <- Response{Error: &ResponseError{Type: "decode_error", Message: fmt.Sprintf("decode response failed: %v", err)}}:
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case res <- chatResponse:
				if chatResponse.StopReason != "" {
					return
				}
			}
		}
	}()

	return res, nil
}

type Request struct {
	// Model The model that will complete your prompt.
	Model Model `json:"model"`
	// MaxTokensToSample The maximum number of tokens to generate before stopping.
	// Note that our models may stop before reaching this maximum.
	// This parameter only specifies the absolute maximum number of tokens to generate.
	MaxTokensToSample int `json:"max_tokens_to_sample"`
	// StopSequences Sequences that will cause the model to stop generating completion text.
	StopSequences []string `json:"stop_sequences,omitempty"`
	// Temperature Amount of randomness injected into the response.
	// Defaults to 1. Ranges from 0 to 1.
	// Use temp closer to 0 for analytical / multiple choice,
	// and closer to 1 for creative and generative tasks.
	Temperature float64 `json:"temperature,omitempty"`
	// TopP Use nucleus sampling.
	TopP float64 `json:"top_p,omitempty"`
	// TopK Only sample from the top K options for each subsequent token.
	TopK float64 `json:"top_k,omitempty"`
	// MetaData An object describing metadata about the request.
	MetaData MetaData `json:"metadata,omitempty"`
	// Stream Whether to incrementally stream the response using server-sent events.
	Stream bool `json:"stream,omitempty"`
	// Prompt The prompt that you want Claude to complete.
	Prompt string `json:"prompt"`
}

type Message struct {
	Role    string
	Content string
}

type Messages []Message

func NewRequest(model Model, messages Messages) Request {
	var prompt string
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			prompt += "\n\nHuman: " + strings.TrimSpace(msg.Content)
		case "assistant":
			prompt += "\n\nAssistant: " + strings.TrimSpace(msg.Content)
		}
	}

	prompt += "\n\nAssistant: "

	return Request{
		Prompt: prompt,
		Model:  model,
	}
}

type MetaData struct {
	// UserID An external identifier for the user who is associated with the request.
	// This should be a uuid, hash value, or other opaque identifier.
	// Anthropic may use this id to help detect abuse.
	// Do not include any identifying information such as name, email address, or phone number.
	UserID string `json:"user_id,omitempty"`
}

type Response struct {
	// Completion The resulting completion up to and excluding the stop sequences.
	Completion string `json:"completion"`
	// StopReason The reason that we stopped sampling.
	// This may be one the following values:
	// - "stop_sequence": we reached a stop sequence — either provided by you via
	// 		the stop_sequences parameter, or a stop sequence built into the model
	// - "max_tokens": we exceeded max_tokens_to_sample or the model's maximum
	StopReason string `json:"stop_reason"`
	// Model The model that performed the completion.
	Model string `json:"model"`

	// Error 错误信息
	Error *ResponseError `json:"error,omitempty"`
}

type ResponseError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
