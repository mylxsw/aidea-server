package search

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestYouComSearch_RequestConstruction(t *testing.T) {
	var gotPath, gotAPIKey, gotQuery, gotCount string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-API-Key")
		gotQuery = r.URL.Query().Get("query")
		gotCount = r.URL.Query().Get("count")

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":{"web":[]}}`)
	}))
	defer server.Close()

	s := newYouComSearchWithBaseURL("test-api-key", nil, server.URL+"/v1/search")
	req := Request{Query: "bitcoin price", ResultCount: 3}

	if _, err := s.Search(context.Background(), &req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/v1/search" {
		t.Errorf("unexpected path: %s", gotPath)
	}
	if gotAPIKey != "test-api-key" {
		t.Errorf("unexpected X-API-Key header: %s", gotAPIKey)
	}
	if gotQuery != "bitcoin price" {
		t.Errorf("unexpected query param: %s", gotQuery)
	}
	if gotCount != "3" {
		t.Errorf("unexpected count param: %s", gotCount)
	}
}

func TestYouComSearch_ResultCountDefaultAndCap(t *testing.T) {
	cases := []struct {
		name        string
		resultCount int
		wantCount   string
	}{
		{"zero uses default", 0, "5"},
		{"negative uses default", -1, "5"},
		{"within range kept as-is", 10, "10"},
		{"over cap clamped to 20", 100, "20"},
		{"exactly cap kept as-is", 20, "20"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotCount string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotCount = r.URL.Query().Get("count")
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"results":{"web":[]}}`)
			}))
			defer server.Close()

			s := newYouComSearchWithBaseURL("key", nil, server.URL)
			req := Request{Query: "q", ResultCount: tc.resultCount}
			if _, err := s.Search(context.Background(), &req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotCount != tc.wantCount {
				t.Errorf("count = %s, want %s", gotCount, tc.wantCount)
			}
		})
	}
}

func TestYouComSearch_MappingWebAndNews(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"results": {
				"web": [
					{"url":"https://example.com/1","title":"Web One","description":"desc one","snippets":["snip a","snip b"]},
					{"url":"https://example.com/2","title":"Web Two","description":"desc two"}
				],
				"news": [
					{"url":"https://news.example.com/1","title":"News One","snippets":["only snippet"]}
				]
			}
		}`)
	}))
	defer server.Close()

	s := newYouComSearchWithBaseURL("key", nil, server.URL)
	resp, err := s.Search(context.Background(), &Request{Query: "q"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Documents) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(resp.Documents))
	}

	d0 := resp.Documents[0]
	if d0.Title != "Web One" || d0.Source != "https://example.com/1" || d0.Index != "1" {
		t.Errorf("unexpected doc 0: %+v", d0)
	}
	if d0.Content != "desc one\nsnip a\nsnip b" {
		t.Errorf("unexpected content assembly for doc 0: %q", d0.Content)
	}

	d1 := resp.Documents[1]
	if d1.Content != "desc two" {
		t.Errorf("unexpected content for doc 1 (no snippets): %q", d1.Content)
	}
	if d1.Index != "2" {
		t.Errorf("unexpected index for doc 1: %s", d1.Index)
	}

	// News is appended after web, and index numbering continues sequentially.
	d2 := resp.Documents[2]
	if d2.Title != "News One" || d2.Source != "https://news.example.com/1" {
		t.Errorf("unexpected news doc: %+v", d2)
	}
	if d2.Index != "3" {
		t.Errorf("unexpected index for news doc: %s", d2.Index)
	}
	if d2.Content != "only snippet" {
		t.Errorf("unexpected content for news doc (description missing): %q", d2.Content)
	}
}

func TestYouComSearch_NewsAbsent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":{"web":[{"url":"https://example.com","title":"T","description":"D"}]}}`)
	}))
	defer server.Close()

	s := newYouComSearchWithBaseURL("key", nil, server.URL)
	resp, err := s.Search(context.Background(), &Request{Query: "q"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(resp.Documents))
	}
}

func TestYouComSearch_OptionalFieldsAllMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":{"web":[{"url":"https://example.com","title":"T"}]}}`)
	}))
	defer server.Close()

	s := newYouComSearchWithBaseURL("key", nil, server.URL)
	resp, err := s.Search(context.Background(), &Request{Query: "q"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(resp.Documents))
	}
	if resp.Documents[0].Content != "" {
		t.Errorf("expected empty content when description/snippets missing, got %q", resp.Documents[0].Content)
	}
}

func TestYouComSearch_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":{}}`)
	}))
	defer server.Close()

	s := newYouComSearchWithBaseURL("key", nil, server.URL)
	resp, err := s.Search(context.Background(), &Request{Query: "q"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Documents) != 0 {
		t.Errorf("expected 0 documents, got %d", len(resp.Documents))
	}
}

func TestYouComSearch_ErrorStatusCodes(t *testing.T) {
	cases := []struct {
		status int
		body   string
	}{
		{http.StatusUnauthorized, `{"error":"invalid api key"}`},
		{http.StatusTooManyRequests, `{"error":"rate limited"}`},
		{http.StatusInternalServerError, `{"error":"internal error"}`},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()

			s := newYouComSearchWithBaseURL("super-secret-key", nil, server.URL)
			_, err := s.Search(context.Background(), &Request{Query: "q"})
			if err == nil {
				t.Fatalf("expected error for status %d, got nil", tc.status)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%d", tc.status)) {
				t.Errorf("expected error to mention status code %d, got: %v", tc.status, err)
			}
			if strings.Contains(err.Error(), "super-secret-key") {
				t.Errorf("error message must never echo the API key: %v", err)
			}
		})
	}
}

func TestYouComSearch_ErrorBodyTruncatedRuneSafe(t *testing.T) {
	// Build a body longer than the truncation limit, using multi-byte runes
	// near the truncation boundary to make sure we never split one in half.
	huge := strings.Repeat("a", youComMaxErrorBodyRunes-1) + "中文超长错误信息用于测试截断" + strings.Repeat("b", 100)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, huge)
	}))
	defer server.Close()

	s := newYouComSearchWithBaseURL("key", nil, server.URL)
	_, err := s.Search(context.Background(), &Request{Query: "q"})
	if err == nil {
		t.Fatal("expected error")
	}

	// Must not panic (rune-safe) and must not contain the full huge body.
	if strings.Contains(err.Error(), strings.Repeat("b", 100)) {
		t.Errorf("expected error body to be truncated, but full tail was present: %v", err)
	}

	// Confirm the string is valid UTF-8 (no split multi-byte rune).
	if !isValidUTF8(err.Error()) {
		t.Errorf("error message is not valid UTF-8, a rune was split: %q", err.Error())
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

func TestYouComSearch_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{not valid json`)
	}))
	defer server.Close()

	s := newYouComSearchWithBaseURL("key", nil, server.URL)
	_, err := s.Search(context.Background(), &Request{Query: "q"})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestYouComSearch_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":{"web":[]}}`)
	}))
	defer server.Close()

	s := newYouComSearchWithBaseURL("key", nil, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := s.Search(ctx, &Request{Query: "q"})
	if err == nil {
		t.Fatal("expected a timeout error")
	}
}

// TestYouComSearch_Search is the live integration test, mirroring the repo's
// existing style (e.g. TestBochaWebSearch_Search): skipped cleanly when
// YDC_API_KEY is not set, and makes exactly one real call otherwise.
func TestYouComSearch_Search(t *testing.T) {
	apiKey := os.Getenv("YDC_API_KEY")
	if apiKey == "" {
		t.Skip("YDC_API_KEY not set, skipping live You.com search test")
	}

	s := NewYouComSearch(apiKey, nil)

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
