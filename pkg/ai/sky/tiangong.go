package sky

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"io"
	"net/http"
	"strings"
	"time"
)

type Sky struct {
	appKey    string
	appSecret string
}

func New(appKey, appSecret string) *Sky {
	return &Sky{appKey: appKey, appSecret: appSecret}
}

const (
	ModelSkyChatMegaVerse = "SkyChat-MegaVerse"
)

type Message struct {
	// Role system,user,bot
	Role    string `json:"role,omitempty"`
	Content string `json:"content"`
}

type Request struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type Response struct {
	Code     int      `json:"code"`      // 错误代码
	CodeMsg  string   `json:"code_msg"`  // 错误描述，发生错误时，错误提示
	TraceId  string   `json:"trace_id"`  // bug 排查
	RespData RespData `json:"resp_data"` // 具体的回复内容在这里
}

type RespData struct {
	Status       int    `json:"status"`
	Reply        string `json:"reply"`
	FinishReason int    `json:"finish_reason"` // 1正常结束，2：token限制
}

func (r RespData) IsSensitive() bool {
	return r.Status == 4 // 4: 敏感词结束
}

func (ai *Sky) Chat(ctx context.Context, req Request) (*Response, error) {
	timestamp := fmt.Sprintf("%v", time.Now().Unix())

	resp, err := misc.RestyClient(2).R().
		SetContext(ctx).
		SetBody(req).
		SetHeader("app_key", ai.appKey).
		SetHeader("timestamp", timestamp).
		SetHeader("sign", misc.Md5([]byte(ai.appKey+ai.appSecret+timestamp))).
		SetHeader("Content-Type", "application/json").
		SetHeader("stream", "false").Post("https://sky-api.singularity-ai.com/saas/api/v4/generate")
	if err != nil {
		return nil, err
	}

	respData := resp.Body()

	var chatResponse Response
	if err := json.Unmarshal(respData, &chatResponse); err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("chat failed, status code: %d, %s", resp.StatusCode(), chatResponse.CodeMsg)
	}

	if chatResponse.Code == 200 {
		chatResponse.Code = 0
	}

	return &chatResponse, nil
}

func (ai *Sky) ChatStream(ctx context.Context, req Request) (<-chan Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://sky-api.singularity-ai.com/saas/api/v4/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	timestamp := fmt.Sprintf("%v", time.Now().Unix())
	httpReq.Header.Set("app_key", ai.appKey)
	httpReq.Header.Set("timestamp", timestamp)
	httpReq.Header.Set("sign", misc.Md5([]byte(ai.appKey+ai.appSecret+timestamp)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("stream", "true")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()

		return nil, fmt.Errorf("chat failed [%s]: %s", httpResp.Status, string(data))
	}

	res := make(chan Response)
	go func() {
		defer func() {
			_ = httpResp.Body.Close()
			close(res)
		}()

		reader := bufio.NewReader(httpResp.Body)
		for {
			data, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return
				}

				select {
				case <-ctx.Done():
				case res <- Response{Code: 100, CodeMsg: fmt.Sprintf("read response failed: %v", err)}:
				}
				return
			}

			dataStr := strings.TrimSpace(string(data))
			if dataStr == "" {
				continue
			}

			var chatResponse Response
			if err := json.Unmarshal([]byte(dataStr), &chatResponse); err != nil {
				select {
				case <-ctx.Done():
				case res <- Response{Code: 100, CodeMsg: fmt.Sprintf("decode response failed: %v", err)}:
				}
				return
			}

			if chatResponse.Code == 200 {
				chatResponse.Code = 0
			}

			select {
			case <-ctx.Done():
				return
			case res <- chatResponse:
				if chatResponse.RespData.FinishReason == 1 {
					return
				}
			}
		}
	}()

	return res, nil
}
