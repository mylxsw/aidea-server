package dashscope

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type DashScope struct {
	apiKey     string
	serviceURL string
}

func New(apiKey string) *DashScope {
	return &DashScope{
		apiKey:     apiKey,
		serviceURL: "https://dashscope.aliyuncs.com",
	}
}

const (
	// 通义千问
	ModelQWenV1     = "qwen-v1"
	ModelQWenPlusV1 = "qwen-plus-v1"

	// 通义千问7B
	ModelQWen7BV1     = "qwen-7b-v1"
	ModelQWen7BChatV1 = "qwen-7b-chat-v1"

	// Llama2
	ModelLlama27BV2      = "llama2-7b-v2"
	ModelLlama27BChatV2  = "llama2-7b-chat-v2"
	ModelLlama213BV2     = "llama2-13b-v2"
	ModelLlama213BChatV2 = "llama2-13b-chat-v2"

	// 百川
	ModelBaiChuan7BV1 = "baichuan-7b-v1"

	// ChatGLM
	ModelChatGLM6BV2 = "chatglm-6b-v2"
)

type ChatRequest struct {
	// Model 指明需要调用的模型，目前可选 qwen-v1 和 qwen-plus-v1
	Model      string         `json:"model,omitempty"`
	Input      ChatInput      `json:"input,omitempty"`
	Parameters ChatParameters `json:"parameters,omitempty"`
}

type ChatInput struct {
	// Prompt 用户当前输入的期望模型执行指令，支持中英文。
	// qwen-v1 prompt字段支持 1.5k Tokens 长度；
	// qwen-plus-v1 prompt字段支持 6.5k Tokens 长度
	Prompt string `json:"prompt,omitempty"`
	// History 用户与模型的对话历史，list中的每个元素是形式为{"user":"用户输入","bot":"模型输出"}的一轮对话，多轮对话按时间正序排列。
	History []ChatHistory `json:"history,omitempty"`
}

type ChatParameters struct {
	// TopP 生成时，核采样方法的概率阈值。例如，取值为0.8时，仅保留累计概率之和大于等于0.8的概率分布中的token，
	// 作为随机采样的候选集。取值范围为(0,1.0)，取值越大，生成的随机性越高；取值越低，生成的随机性越低。
	// 默认值 0.8。注意，取值不要大于等于1
	TopP float64 `json:"top_p,omitempty"`
	// TopK 生成时，采样候选集的大小。例如，取值为50时，仅将单次生成中得分最高的50个token组成随机采样的候选集。
	// 取值越大，生成的随机性越高；取值越小，生成的确定性越高。注意：如果top_k的值大于100，top_k将采用默认值100
	TopK int `json:"top_k,omitempty"`
	// Seed 生成时，随机数的种子，用于控制模型生成的随机性。如果使用相同的种子，每次运行生成的结果都将相同；
	// 当需要复现模型的生成结果时，可以使用相同的种子。seed参数支持无符号64位整数类型。默认值 1234
	Seed int `json:"seed,omitempty"`
	// EnableSearch 生成时，是否参考夸克搜索的结果。注意：打开搜索并不意味着一定会使用搜索结果；
	// 如果打开搜索，模型会将搜索结果作为prompt，进而“自行判断”是否生成结合搜索结果的文本，默认为false
	EnableSearch bool `json:"enable_search,omitempty"`
}

type ChatHistory struct {
	User string `json:"user,omitempty"`
	Bot  string `json:"bot,omitempty"`
}

type ChatResponse struct {
	// RequestID 本次请求的系统唯一码
	RequestID string     `json:"request_id,omitempty"`
	Output    ChatOutput `json:"output,omitempty"`
	Usage     ChatUsage  `json:"usage,omitempty"`
	Code      string     `json:"code,omitempty"`
	Message   string     `json:"message,omitempty"`
}

func (res ChatResponse) IsStopped() bool {
	return res.Output.FinishReason != ""
}

type ChatOutput struct {
	// Text 本次请求的算法输出内容。
	Text string `json:"text,omitempty"`
	// FinishReason 有三种情况：
	//	正在生成时为 null
	//	生成结束时如果由于停止token导致则为 stop
	//	生成结束时如果因为生成长度过长导致则为 length
	FinishReason string `json:"finish_reason,omitempty"`
}

const (
	FinishReasonStop   = "stop"
	FinishReasonLength = "length"
)

type ChatUsage struct {
	// OutputTokens 本次请求的算法输出的token数量
	OutputTokens int `json:"output_tokens,omitempty"`
	// InputTokens 本次请求输入内容的 token 数目。在打开了搜索的情况下，输入的 token 数目
	// 因为还需要添加搜索相关内容支持，所以会超出客户在请求中的输入
	InputTokens int `json:"input_tokens,omitempty"`
}

func (ds *DashScope) Chat(req ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", ds.serviceURL+"/api/v1/services/aigc/text-generation/generation", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+ds.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-DashScope-DataInspection", "enable")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	defer httpResp.Body.Close()

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("chat failed [%d]: %s", httpResp.StatusCode, httpResp.Status)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	return &chatResp, nil
}

func (ds *DashScope) ChatStream(req ChatRequest) (<-chan ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", ds.serviceURL+"/api/v1/services/aigc/text-generation/generation", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+ds.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")
	httpReq.Header.Set("X-DashScope-SSE", "enable")
	httpReq.Header.Set("X-DashScope-DataInspection", "enable")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		_ = httpResp.Body.Close()
		return nil, fmt.Errorf("chat failed [%d]: %s", httpResp.StatusCode, httpResp.Status)
	}

	res := make(chan ChatResponse)
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

				res <- ChatResponse{Message: fmt.Sprintf("read stream failed: %s", err.Error()), Code: "READ_STREAM_FAILED"}
				return
			}

			dataStr := strings.TrimSpace(string(data))
			if dataStr == "" {
				continue
			}

			if !strings.HasPrefix(dataStr, "data:") {
				//id:1
				//event:result
				//data:...
				continue
			}

			var chatResponse ChatResponse
			if err := json.Unmarshal([]byte(dataStr[5:]), &chatResponse); err != nil {
				res <- ChatResponse{Message: fmt.Sprintf("unmarshal stream data failed: %v", err), Code: "UNMARSHAL_STREAM_DATA_FAILED"}
				return
			}

			res <- chatResponse
			if chatResponse.Output.FinishReason != "" && chatResponse.Output.FinishReason != "null" {
				return
			}
		}
	}()

	return res, nil
}
