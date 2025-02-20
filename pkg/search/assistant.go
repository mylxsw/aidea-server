package search

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	oai "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/sashabaranov/go-openai"
)

type SearchAssistant struct {
	client oai.Client
	model  string
}

func NewSearchAssistant(client oai.Client, model string) *SearchAssistant {
	return &SearchAssistant{client: client, model: model}
}

const serachQueryPrompt = `You are an expert search query generator, tasked with creating optimized search queries for use in search engines. Your goal is to generate a precise and effective query based on a user's question and their recent chat history.

Here is the user's question:
<user_question>
%s
</user_question>

Here is the user's recent chat history:
<chat_history>
%s
</chat_history>

Please generate an optimized search query following these guidelines:

1. The query must be in the same language as the user's question.
2. Include as much relevant information from the user's question as possible, without omitting key details.
3. Consider the user's search history to provide context and improve relevance.
4. The query must not exceed 100 characters in length.
5. Focus on creating a query that will yield the most relevant results in a search engine.

Before providing the final query, wrap your thought process inside <query_formulation> tags:

1. Analyze the user's question and identify key concepts and keywords.
2. List out potential keywords and phrases from the user's question.
3. Review the search history for relevant context and identify any patterns or recurring themes.
4. Formulate an initial query.
5. Consider different query formulations and compare their potential effectiveness.
6. Refine the query to ensure it meets all requirements (language, length, relevance).

After your analysis, provide only the final search query without any additional explanation.

Example output structure:

<query_formulation>
[Your detailed thought process]
</query_formulation>

[Final search query]

Please proceed with generating the optimized search query.
`

func (s *SearchAssistant) GenerateSearchQuery(ctx context.Context, query string, histories []History) (keyword string, err error) {
	if s.client == nil {
		return query, nil
	}

	if len(histories) == 0 && misc.WordCount(query) < 100 {
		return query, nil
	}

	defer func() {
		if err != nil {
			log.WithFields(log.Fields{
				"query":     query,
				"histories": histories,
				"error":     err,
			}).Errorf("generate search query failed")

			keyword = query
			err = nil
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	chatHistory := array.Reduce(histories, func(carry string, item History) string {
		return carry + fmt.Sprintf("\n%s: %s\n---", item.Role, item.Content)
	}, "")

	prompt := fmt.Sprintf(serachQueryPrompt, query, chatHistory)
	req := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.WithFields(log.Fields{
			"query":     query,
			"histories": histories,
			"error":     err,
		}).Errorf("generate search query failed")

		return "", err
	}

	content := array.Reduce(
		resp.Choices,
		func(carry string, item openai.ChatCompletionChoice) string {
			return carry + "\n" + item.Message.Content
		},
		"",
	)

	// Remove the query formulation section (analysis details) from the response, keeping only the final search query
	content = regexp.MustCompile(`(?s)<query_formulation>.*?</query_formulation>`).ReplaceAllString(content, "")
	keyword = strings.TrimSpace(content)

	return
}
