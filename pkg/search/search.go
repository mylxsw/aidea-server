package search

import (
	"context"
	"fmt"

	"github.com/mylxsw/aidea-server/config"
)

type Request struct {
	Query       string    `json:"query,omitempty"`
	Histories   []History `json:"histories,omitempty"`
	ResultCount int       `json:"result_count,omitempty"`
}

type History struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type Response struct {
	Documents []Document `json:"documents,omitempty"`
}

func (resp *Response) ToMessage(limit int) (string, []Document) {
	result := ""
	for i, doc := range resp.Documents[:limit] {
		result += fmt.Sprintf("[webpage %d begin]\nurl: %s\ntitle: %s\ncontent: %s\n[webpage %d end]\n", i+1, doc.Source, doc.Title, doc.Content, i+1)
	}

	return result, resp.Documents[:limit]
}

type Document struct {
	Source  string `json:"source,omitempty"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
}

type Searcher interface {
	Search(ctx context.Context, req *Request) (*Response, error)
}

type searchEngine struct {
	conf      *config.Config
	assistant *SearchAssistant
}

func NewSearcher(conf *config.Config, assistant *SearchAssistant) Searcher {
	return &searchEngine{
		conf:      conf,
		assistant: assistant,
	}
}

func (s *searchEngine) Search(ctx context.Context, req *Request) (*Response, error) {
	if len(req.Histories) > 0 {
		lastHistory := req.Histories[len(req.Histories)-1]
		if lastHistory.Role == "user" && lastHistory.Content == req.Query {
			req.Histories = req.Histories[:len(req.Histories)-1]
		}
	}

	switch s.conf.SearchEngine {
	case "bigmodel":
		return NewBigModelSearch(s.conf.BigModelSearchAPIKey).Search(ctx, req)
	case "bochaai":
		return NewBochaaiSearch(s.conf.BochaaiSearchAPIKey, s.assistant).Search(ctx, req)
	default:
	}

	return &Response{}, nil
}
