package search

import (
	"context"
	"fmt"
	"github.com/mylxsw/go-utils/must"
	"os"
	"testing"
)

func TestBigModelSearch_Search(t *testing.T) {
	s := NewBigModelSearch(os.Getenv("BIGMODEL_API_KEY"))

	req := Request{
		Query:       "现在比特币价格多少？",
		ResultCount: 3,
	}

	resp := must.Must(s.Search(context.TODO(), &req))

	for _, doc := range resp.Documents {
		fmt.Printf("source: %s, title: %s, content: %s\n-------------------\n", doc.Source, doc.Title, doc.Content)
	}
}
