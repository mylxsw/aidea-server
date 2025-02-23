package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/go-utils/array"
	"io"
	"net/http"
	"time"
)

type BochaAISearch struct {
	apiKey    string
	assistant *SearchAssistant
}

func NewBochaAISearch(apiKey string, assistant *SearchAssistant) *BochaAISearch {
	return &BochaAISearch{apiKey: apiKey, assistant: assistant}
}

func (b *BochaAISearch) Search(ctx context.Context, req *Request) (*Response, error) {
	keyword := req.Query
	if b.assistant != nil {
		keyword, _ = b.assistant.GenerateSearchQuery(ctx, req.Query, req.Histories)
	}

	data := map[string]any{
		"query":     keyword,
		"freshness": "noLimit",
		"count":     req.ResultCount,
		"answer":    false,
		"stream":    false,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.bochaai.com/v1/ai-search", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b.apiKey))

	client := &http.Client{Timeout: 30 * time.Second}
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
	//	log.Debugf("bochaai search response: %s", respBody)
	//}

	var apiResp BochaAISearchResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("API error: %s", *apiResp.Msg)
	}

	items := array.Filter(apiResp.Messages, func(item BochaAIMessage, _ int) bool {
		return item.ContentType == "webpage"
	})

	var documents []Document
	var index int
	for _, item := range items {
		var webPage BochaAIWebSearch
		if err := json.Unmarshal([]byte(item.Content), &webPage); err != nil {
			continue
		}

		for _, value := range webPage.Value {
			documents = append(documents, Document{
				Source:  value.URL,
				Title:   value.Name,
				Content: value.Summary,
				Icon:    value.SiteIcon,
				Media:   value.SiteName,
				Index:   fmt.Sprintf("%d", index+1),
			})
			index++
		}
	}

	return &Response{Documents: documents}, nil
}

type BochaAISearchResponse struct {
	Code           int              `json:"code"`
	LogID          string           `json:"log_id"`
	ConversationID string           `json:"conversation_id"`
	Messages       []BochaAIMessage `json:"messages"`
	Msg            *string          `json:"msg"`
}

type BochaAIMessage struct {
	Role        string `json:"role"`
	Type        string `json:"type"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
}

type BochaAIWebSearch struct {
	SomeResultsRemoved bool `json:"someResultsRemoved"`
	Value              []struct {
		DateLastCrawled string `json:"dateLastCrawled"`
		DisplayUrl      string `json:"displayUrl"`
		ID              string `json:"id"`
		Name            string `json:"name"`
		SiteIcon        string `json:"siteIcon"`
		SiteName        string `json:"siteName"`
		Snippet         string `json:"snippet"`
		Summary         string `json:"summary"`
		URL             string `json:"url"`
	} `json:"value"`
	WebSearchUrl string `json:"webSearchUrl"`
}
