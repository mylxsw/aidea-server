package search

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestBochaWebSearch_Search(t *testing.T) {
	s := NewBochaWebSearch(os.Getenv("BOCHAAI_API_KEY"), nil)

	req := Request{
		Query:       "现在比特币价格多少？",
		ResultCount: 3,
	}

	resp, err := s.Search(context.Background(), &req)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	for _, doc := range resp.Documents {
		fmt.Printf("source: %s, title: %s, content: %s\n-------------------\n", doc.Source, doc.Title, doc.Content)
	}
}
