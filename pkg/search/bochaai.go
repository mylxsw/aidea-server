package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type BochaaiSearch struct {
	apiKey    string
	assistant *SearchAssistant
}

func NewBochaaiSearch(apiKey string, assistant *SearchAssistant) *BochaaiSearch {
	return &BochaaiSearch{apiKey: apiKey, assistant: assistant}
}

func (b *BochaaiSearch) Search(ctx context.Context, req *Request) (*Response, error) {
	keyword := req.Query
	if b.assistant != nil {
		keyword, _ = b.assistant.GenerateSearchQuery(ctx, req.Query, req.Histories)
	}

	data := map[string]any{
		"query":     keyword,
		"freshness": "noLimit",
		"summary":   true,
		"count":     req.ResultCount,
		"page":      1,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.bochaai.com/v1/web-search", bytes.NewBuffer(jsonData))
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

	var apiResp BochAAISearchResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("API error: %s", *apiResp.Msg)
	}

	var documents []Document
	for i, result := range apiResp.Data.WebPages.Value {
		documents = append(documents, Document{
			Title:   result.Name,
			Source:  result.URL,
			Content: result.Summary,
			Icon:    result.SiteIcon,
			Media:   result.SiteName,
			Index:   fmt.Sprintf("%d", i+1),
		})
	}

	return &Response{Documents: documents}, nil
}

type BochAAISearchResponse struct {
	Code  int             `json:"code,omitempty"`
	LogID string          `json:"log_id,omitempty"`
	Msg   *string         `json:"msg,omitempty"`
	Data  BochAAIDataResp `json:"data,omitempty"`
}

type BochAAIDataResp struct {
	Type         string              `json:"_type,omitempty"`
	QueryContext BochAAIQueryContext `json:"queryContext,omitempty"`
	WebPages     BochAAIWebPages     `json:"webPages,omitempty"`
}

type BochAAIQueryContext struct {
	OriginalQuery string `json:"originalQuery,omitempty"`
}

type BochAAIWebPages struct {
	WebSearchURL          string           `json:"webSearchUrl,omitempty"`
	TotalEstimatedMatches int              `json:"totalEstimatedMatches,omitempty"`
	Value                 []BochAAIWebPage `json:"value,omitempty"`
	SomeResultsRemoved    bool             `json:"someResultsRemoved,omitempty"`
}

type BochAAIWebPage struct {
	ID               string  `json:"id,omitempty"`
	Name             string  `json:"name,omitempty"`
	URL              string  `json:"url,omitempty"`
	DisplayURL       string  `json:"displayUrl,omitempty"`
	Snippet          string  `json:"snippet,omitempty"`
	Summary          string  `json:"summary,omitempty"`
	SiteName         string  `json:"siteName,omitempty"`
	SiteIcon         string  `json:"siteIcon,omitempty"`
	DateLastCrawled  string  `json:"dateLastCrawled,omitempty"`
	CachedPageURL    *string `json:"cachedPageUrl,omitempty"`
	Language         *string `json:"language,omitempty"`
	IsFamilyFriendly *bool   `json:"isFamilyFriendly,omitempty"`
	IsNavigational   *bool   `json:"isNavigational,omitempty"`
}
