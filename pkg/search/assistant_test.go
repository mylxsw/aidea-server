package search

import (
	"context"
	"os"
	"testing"

	oai "github.com/mylxsw/aidea-server/pkg/ai/openai"
)

func TestSearchAssistant_GenerateSearchQuery(t *testing.T) {
	conf := oai.Config{
		Enable:        true,
		OpenAIKeys:    []string{os.Getenv("OPENAI_API_KEY")},
		OpenAIServers: []string{"https://api.openai.com/v1"},
	}
	client := oai.NewOpenAIClient(&conf, nil)

	assistant := NewSearchAssistant(client, "gpt-4o-mini")
	query := "现在的价格是多少？"
	histories := []History{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "你好，我是小助手，请问有什么可以帮你的？"},
		{Role: "user", Content: "你知道比特币吗"},
		{Role: "assistant", Content: "比特币是一种数字货币，它是由一个叫做中本聪的人发明的。"},
	}

	query, err := assistant.GenerateSearchQuery(context.Background(), query, histories)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(query)
}
