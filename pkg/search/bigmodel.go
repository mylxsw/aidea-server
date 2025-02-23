package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type BigModelSearch struct {
	apiKey string
}

func NewBigModelSearch(apiKey string) *BigModelSearch {
	return &BigModelSearch{apiKey: apiKey}
}

// Search performs a search using the BigModel API.
func (b *BigModelSearch) Search(ctx context.Context, req *Request) (*Response, error) {
	url := "https://open.bigmodel.cn/api/paas/v4/tools"
	requestID := uuid.New().String()
	data := map[string]any{
		"request_id": requestID,
		"tool":       "web-search-pro",
		"stream":     false,
		"messages": append(req.Histories, History{
			Role:    "user",
			Content: req.Query,
		}),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", b.apiKey)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error: status code %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	//if log.DebugEnabled() {
	//	log.WithFields(data).Debugf("bigModel search response: %s", respBody)
	//}

	var apiResp BigModelSearchResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Choices) == 0 || len(apiResp.Choices[0].Message.ToolCalls) == 0 {
		return &Response{}, nil
	}

	var documents []Document
	var index int
	for _, choice := range apiResp.Choices {
		for _, toolCall := range choice.Message.ToolCalls {
			if toolCall.Type != "search_result" {
				continue
			}

			for _, result := range toolCall.SearchResult {
				if result.Content == "" {
					continue
				}

				documents = append(documents, Document{
					Content: result.Content,
					Source:  result.Link,
					Title:   result.Title,
					Icon:    result.Icon,
					Media:   result.Media,
					Index:   fmt.Sprintf("%d", index+1),
				})
				index++
			}
		}
	}

	return &Response{Documents: documents}, nil
}

type BigModelSearchResponse struct {
	Choices   []BigModelSearchChoice `json:"choices,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Model     string                 `json:"model,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Usage     BigModelSearchUsage    `json:"usage,omitempty"`
}

type BigModelSearchUsage struct {
	CompletionTokens int `json:"completion_tokens,omitempty"`
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type BigModelSearchChoice struct {
	FinishReason string                `json:"finish_reason,omitempty"`
	Index        int                   `json:"index,omitempty"`
	Message      BigModelSearchMessage `json:"message,omitempty"`
}

type BigModelSearchMessage struct {
	Role      string                   `json:"role,omitempty"`
	ToolCalls []BigModelSearchToolCall `json:"tool_calls,omitempty"`
}

type BigModelSearchToolCall struct {
	ID           string                 `json:"id,omitempty"`
	Type         string                 `json:"type,omitempty"`
	SearchIntent []BigModelSearchIntent `json:"search_intent,omitempty"`
	SearchResult []BigModelSearchResult `json:"search_result,omitempty"`
}

type BigModelSearchIntent struct {
	Category string `json:"category,omitempty"`
	Index    int    `json:"index,omitempty"`
	Intent   string `json:"intent,omitempty"`
	Keywords string `json:"keywords,omitempty"`
	Query    string `json:"query,omitempty"`
}

type BigModelSearchResult struct {
	Content string `json:"content,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Index   int    `json:"index,omitempty"`
	Link    string `json:"link,omitempty"`
	Media   string `json:"media,omitempty"`
	Refer   string `json:"refer,omitempty"`
	Title   string `json:"title,omitempty"`
}
