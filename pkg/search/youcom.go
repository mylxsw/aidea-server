package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// youComDefaultBaseURL is the production You.com search endpoint.
const youComDefaultBaseURL = "https://ydc-index.io/v1/search"

// youComMaxErrorBodyRunes bounds how much of an error response body we echo
// back, so a huge or malformed error page never floods logs/messages.
const youComMaxErrorBodyRunes = 500

type YouComSearch struct {
	apiKey    string
	assistant *SearchAssistant
	baseURL   string
}

// NewYouComSearch creates a You.com web search provider using the production endpoint.
func NewYouComSearch(apiKey string, assistant *SearchAssistant) *YouComSearch {
	return newYouComSearchWithBaseURL(apiKey, assistant, youComDefaultBaseURL)
}

// newYouComSearchWithBaseURL allows tests to point the provider at a local httptest server.
func newYouComSearchWithBaseURL(apiKey string, assistant *SearchAssistant, baseURL string) *YouComSearch {
	return &YouComSearch{apiKey: apiKey, assistant: assistant, baseURL: baseURL}
}

func (y *YouComSearch) Search(ctx context.Context, req *Request) (*Response, error) {
	keyword := req.Query
	if y.assistant != nil && len(req.Histories) > 0 {
		keyword, _ = y.assistant.GenerateSearchQuery(ctx, req.Query, req.Histories)
	}

	count := req.ResultCount
	if count <= 0 {
		count = 5
	}
	if count > 20 {
		count = 20
	}

	query := url.Values{}
	query.Set("query", keyword)
	query.Set("count", fmt.Sprintf("%d", count))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", y.baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-API-Key", y.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error: status code %d, body: %s", resp.StatusCode, truncateRunes(string(respBody), youComMaxErrorBodyRunes))
	}

	var apiResp YouComSearchResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, err
	}

	var documents []Document
	index := 0
	for _, result := range apiResp.Results.Web {
		index++
		documents = append(documents, Document{
			Title:   result.Title,
			Source:  result.URL,
			Content: youComResultContent(result),
			Index:   fmt.Sprintf("%d", index),
		})
	}
	for _, result := range apiResp.Results.News {
		index++
		documents = append(documents, Document{
			Title:   result.Title,
			Source:  result.URL,
			Content: youComResultContent(result),
			Index:   fmt.Sprintf("%d", index),
		})
	}

	return &Response{Documents: documents}, nil
}

// youComResultContent assembles the Document content from a result's
// description and snippets, both of which are optional.
func youComResultContent(result YouComSearchResult) string {
	content := result.Description
	if len(result.Snippets) > 0 {
		snippets := ""
		for i, snippet := range result.Snippets {
			if i > 0 {
				snippets += "\n"
			}
			snippets += snippet
		}

		if content != "" {
			content += "\n" + snippets
		} else {
			content = snippets
		}
	}

	return content
}

// truncateRunes truncates s to at most n runes, rune-safe (never splits a
// multi-byte character in half).
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

type YouComSearchResponse struct {
	Results YouComSearchResults `json:"results,omitempty"`
}

type YouComSearchResults struct {
	Web  []YouComSearchResult `json:"web,omitempty"`
	News []YouComSearchResult `json:"news,omitempty"`
}

type YouComSearchResult struct {
	URL         string   `json:"url,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Snippets    []string `json:"snippets,omitempty"`
}
