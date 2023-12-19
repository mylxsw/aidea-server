package google

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/bcicen/jstream"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/proxy"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/array"
	"gopkg.in/resty.v1"
	"io"
	"net/http"
	"time"
)

const (
	RoleUser  = "user"
	RoleModel = "model"
)

const (
	ModelGeminiPro       = "gemini-pro"
	ModelGeminiProVision = "gemini-pro-vision"
)

type GoogleAI struct {
	serverURL string
	apiKey    string

	client *http.Client
	resty  *resty.Client
}

func newGoogleAI(resolver infra.Resolver, conf *config.Config) *GoogleAI {
	client := &http.Client{Timeout: 180 * time.Second}
	restyClient := misc.RestyClient(2).SetTimeout(180 * time.Second)

	if conf.SupportProxy() && conf.GoogleAIAutoProxy {
		resolver.MustResolve(func(pp *proxy.Proxy) {
			transport := pp.BuildTransport()
			client.Transport = transport
			restyClient.SetTransport(transport)
		})
	}

	return &GoogleAI{
		serverURL: conf.GoogleAIServer,
		apiKey:    conf.GoogleAIKey,
		client:    client,
		resty:     restyClient,
	}
}

func NewGoogleAI(serverURL string, apiKey string) *GoogleAI {
	if serverURL == "" {
		serverURL = "https://generativelanguage.googleapis.com"
	}

	return &GoogleAI{
		serverURL: serverURL,
		apiKey:    apiKey,
		client:    http.DefaultClient,
		resty:     misc.RestyClient(2).SetTimeout(180 * time.Second),
	}
}

type Request struct {
	Contents         []Message         `json:"contents,omitempty"`
	SafetySettings   []SafetySetting   `json:"safetySettings,omitempty"`
	GenerationConfig *GenerationConfig `json:"generationConfig,omitempty"`
}

func (req *Request) HasImage() bool {
	for _, content := range req.Contents {
		for _, part := range content.Parts {
			if part.InlineData != nil {
				return true
			}
		}
	}

	return false
}

type SafetySetting struct {
	Category  string `json:"category,omitempty"`
	Threshold string `json:"threshold,omitempty"`
}

type GenerationConfig struct {
	StopSequences   []string `json:"stopSequences,omitempty"`
	Temperature     float64  `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	TopP            float64  `json:"topP,omitempty"`
	TopK            int      `json:"topK,omitempty"`
}

type Message struct {
	Role  string        `json:"role,omitempty"`
	Parts []MessagePart `json:"parts,omitempty"`
}

type MessagePart struct {
	Text       string                 `json:"text,omitempty"`
	InlineData *MessagePartInlineData `json:"inlineData,omitempty"`
}

type MessagePartInlineData struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

type Response struct {
	Candidates     []Candidate     `json:"candidates,omitempty"`
	PromptFeedback *PromptFeedback `json:"promptFeedback,omitempty"`
	Error          *ErrorResponse  `json:"error,omitempty"`
}

func (resp *Response) String() string {
	return array.Reduce(resp.Candidates, func(carry string, item Candidate) string {
		return carry + array.Reduce(
			item.Content.Parts,
			func(carry string, item MessagePart) string { return carry + item.Text },
			"",
		)
	}, "")
}

type Candidate struct {
	Content       Message        `json:"content,omitempty"`
	FinishReason  string         `json:"finishReason,omitempty"`
	Index         int            `json:"index,omitempty"`
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
}

type SafetyRating struct {
	Category    string `json:"category,omitempty"`
	Probability string `json:"probability,omitempty"`
}

type PromptFeedback struct {
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
}

type ErrorResponse struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
}

func (ai *GoogleAI) Chat(ctx context.Context, model string, req Request) (*Response, error) {
	resp, err := ai.resty.R().
		SetQueryParam("key", ai.apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		SetContext(ctx).
		Post(fmt.Sprintf("%s/v1beta/models/%s:generateContent", ai.serverURL, model))
	if err != nil {
		return nil, err
	}

	respData := resp.Body()

	var chatResponse Response
	if err := json.Unmarshal(respData, &chatResponse); err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("chat failed, status code: %d, %s", resp.StatusCode(), chatResponse.Error.Message)
	}

	return &chatResponse, nil
}

func (ai *GoogleAI) ChatStream(ctx context.Context, model string, req Request) (<-chan Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s", ai.serverURL, model, ai.apiKey),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	httpResp, err := ai.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(httpResp.Body)
		var chatResponse []Response
		if err := json.Unmarshal(respBody, &chatResponse); err != nil {
			return nil, err
		}

		if len(chatResponse) == 0 {
			return nil, fmt.Errorf("chat failed, status code: %d", httpResp.StatusCode)
		}

		return nil, fmt.Errorf("chat failed, status code: %d, %s", httpResp.StatusCode, chatResponse[0].Error.Message)
	}

	res := make(chan Response)
	go func() {
		defer func() {
			_ = httpResp.Body.Close()
			close(res)
		}()

		reader := bufio.NewReader(httpResp.Body)
		decoder := jstream.NewDecoder(reader, 1)
		for obj := range decoder.EmitKV().Stream() {
			data, _ := json.Marshal(obj.Value)
			var ret Response
			_ = json.Unmarshal(data, &ret)

			select {
			case <-ctx.Done():
			case res <- ret:
			}
		}
	}()

	return res, nil
}
