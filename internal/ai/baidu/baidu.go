package baidu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"gopkg.in/resty.v1"
)

type BaiduAI interface {
	Chat(ctx context.Context, model Model, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, model Model, req ChatRequest) (<-chan ChatResponse, error)
}

type BaiduAIImpl struct {
	APIKey      string
	APISecret   string
	accessToken string
	lock        sync.RWMutex
}

func NewBaiduAI(apiKey, apiSecret string) *BaiduAIImpl {
	ai := &BaiduAIImpl{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}

	if err := ai.RefreshAccessToken(); err != nil {
		log.Errorf("refresh baidu ai access token failed: %s", err)
	}

	return ai
}

type RefreshAccessTokenResponse struct {
	RefreshToken  string `json:"refresh_token,omitempty"`
	ExpiresIn     int    `json:"expires_in,omitempty"`
	SessionKey    string `json:"session_key,omitempty"`
	AccessToken   string `json:"access_token,omitempty"`
	Scope         string `json:"scope,omitempty"`
	SessionSecret string `json:"session_secret,omitempty"`
}

// RefreshAccessToken 刷新 AccessToken
func (ai *BaiduAIImpl) RefreshAccessToken() error {
	resp, err := resty.R().
		SetQueryParam("grant_type", "client_credentials").
		SetQueryParam("client_id", ai.APIKey).
		SetQueryParam("client_secret", ai.APISecret).
		Post("https://aip.baidubce.com/oauth/2.0/token")
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("refresh access token failed, status code: %d", resp.StatusCode())
	}

	var accessTokenResponse RefreshAccessTokenResponse
	if err := json.Unmarshal(resp.Body(), &accessTokenResponse); err != nil {
		return err
	}

	if accessTokenResponse.AccessToken != "" {
		ai.lock.Lock()
		ai.accessToken = accessTokenResponse.AccessToken
		ai.lock.Unlock()
	}

	return nil
}

func (ai *BaiduAIImpl) getAccessToken() string {
	ai.lock.RLock()
	defer ai.lock.RUnlock()
	return ai.accessToken
}

type ChatRequest struct {
	// Messages 聊天上下文信息。说明：
	//    （1）messages成员不能为空，1个成员表示单轮对话，多个成员表示多轮对话
	//    （2）最后一个message为当前请求的信息，前面的message为历史对话信息
	//    （3）必须为奇数个成员，成员中message的role必须依次为user、assistant
	//    （4）最后一个message的content长度（即此轮对话的问题）不能超过2000个字符；如果messages中content总长度大于2000字符，系统会依次遗忘最早的历史会话，直到content的总长度不超过2000个字符
	Messages ChatMessages `json:"messages"`
	// Temperature 说明：
	//    （1）较高的数值会使输出更加随机，而较低的数值会使其更加集中和确定
	//    （2）默认0.95，范围 (0, 1.0]，不能为0
	//    （3）建议该参数和top_p只设置1个
	//    （4）建议top_p和temperature不要同时更改
	Temperature float64 `json:"temperature,omitempty"`
	// TopP 说明：
	//    （1）影响输出文本的多样性，取值越大，生成文本的多样性越强
	//    （2）默认0.8，取值范围 [0, 1.0]
	//    （3）建议该参数和temperature只设置1个
	//    （4）建议top_p和temperature不要同时更改
	TopP float64 `json:"top_p,omitempty"`
	// PenaltyScore 通过对已生成的token增加惩罚，减少重复生成的现象。说明：
	//    （1）值越大表示惩罚越大
	//    （2）默认1.0，取值范围：[1.0, 2.0]
	PenaltyScore float64 `json:"penalty_score,omitempty"`
	// Stream 是否以流式接口的形式返回数据，默认false
	Stream bool `json:"stream,omitempty"`
	// UserID 表示最终用户的唯一标识符，可以监视和检测滥用行为，防止接口恶意调用
	UserID string `json:"user_id,omitempty"`
	// System 模型人设，主要用于人设设定，例如，你是xxx公司制作的AI助手，说明：
	// （1）长度限制1024个字符
	// （2）如果使用functions参数，不支持设定人设system
	System string `json:"system,omitempty"`
}

func (req ChatRequest) Fix(model Model) ChatRequest {
	req.Messages = req.Messages.Fix()
	return req
}

const (
	ChatMessageRoleUser      = "user"
	ChatMessageRoleAssistant = "assistant"
)

type ChatMessages []ChatMessage

func (ms ChatMessages) Fix() ChatMessages {
	last := ms[len(ms)-1]
	if last.Role != ChatMessageRoleUser {
		last = ChatMessage{
			Role:    ChatMessageRoleUser,
			Content: "继续",
		}
		ms = append(ms, last)
	}

	finalMessages := make([]ChatMessage, 0)
	var lastRole string

	for _, m := range array.Reverse(ms) {
		if m.Role == lastRole {
			continue
		}

		lastRole = m.Role
		finalMessages = append(finalMessages, m)
	}

	if len(finalMessages)%2 == 0 {
		finalMessages = finalMessages[:len(finalMessages)-1]
	}

	return array.Reverse(finalMessages)
}

type ChatMessage struct {
	// Role 当前支持以下：
	//   user: 表示用户
	//   assistant: 表示对话助手
	Role string `json:"role,omitempty"`
	// Content 对话内容，不能为空
	Content string `json:"content,omitempty"`
}

type ChatResponse struct {
	ErrorCode    int    `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_msg,omitempty"`

	Id string `json:"id,omitempty"`
	// 回包类型 chat.completion：多轮对话返回
	Object string `json:"object,omitempty"`
	// Created 时间戳
	Created int `json:"created,omitempty"`
	// SentenceID 表示当前子句的序号。只有在流式接口模式下会返回该字段
	SentenceID int `json:"sentence_id,omitempty"`
	// IsEND 表示当前子句是否是最后一句。只有在流式接口模式下会返回该字段
	IsEND bool `json:"is_end,omitempty"`
	// Result 对话返回结果
	Result string `json:"result,omitempty"`
	// IsTruncated 当前生成的结果是否被截断
	IsTruncated bool `json:"is_truncated,omitempty"`
	// NeedClearHistory 表示用户输入是否存在安全，是否关闭当前会话，清理历史回话信息
	//		true：是，表示用户输入存在安全风险，建议关闭当前会话，清理历史会话信息
	//		false：否，表示用户输入无安全风险
	NeedClearHistory bool `json:"need_clear_history,omitempty"`
	// BanRound 当need_clear_history为true时，此字段会告知第几轮对话有敏感信息，如果是当前问题，ban_round=-1
	BanRound int `json:"ban_round,omitempty"`
	// Usage token统计信息，token数 = 汉字数+单词数*1.3 （仅为估算逻辑）
	Usage Usage `json:"usage,omitempty"`
}

type Usage struct {
	// PromptTokens 问题 tokens 数
	PromptTokens int `json:"prompt_tokens,omitempty"`
	// CompletionTokens 回答 tokens 数
	CompletionTokens int `json:"completion_tokens,omitempty"`
	// TotalTokens tokens 总数
	TotalTokens int `json:"total_tokens,omitempty"`
}

// SupportSystemMessage 是否支持系统消息
func SupportSystemMessage(model Model) bool {
	return model == ModelErnieBot || model == ModelErnieBotTurbo || model == ModelErnieBot4
}

type Model string

const (
	// ModelErnieBot ERNIE-Bot是百度自行研发的大语言模型，覆盖海量中文数据，具有更强的对话问答、内容创作生成等能力。
	// ¥0.012元/千tokens
	ModelErnieBot Model = "model_ernie_bot"
	// ModelErnieBotTurbo ERNIE-Bot-turbo是百度自行研发的大语言模型，覆盖海量中文数据，具有更强的对话问答、内容创作生成等能力，响应速度更快。
	// ¥0.008元/千tokens
	ModelErnieBotTurbo = "model_ernie_bot_turbo"
	// ModelErnieBot4 文心一言 4.0
	// ¥0.12元/千tokens
	ModelErnieBot4 = "model_ernie_bot_4"
	// ModelLlama2_70b Llama-2-70b-chat由Meta AI研发并开源，在编码、推理及知识应用等场景表现优秀
	// ¥0.044元/千tokens
	ModelLlama2_70b = "model_badiu_llama2_70b"
	// ModelLlama2_7b_CN Qianfan-Chinese-Llama-2-7B是千帆团队在Llama-2-7b基础上的中文增强版本，在CMMLU、C-EVAL等中文数据集上表现优异
	// ¥0.006元/千tokens
	ModelLlama2_7b_CN = "model_baidu_llama2_7b_cn"
	// ModelChatGLM2_6B_32K ChatGLM2-6B是由智谱AI与清华KEG实验室发布的中英双语对话模型，具备强大的推理性能、效果、较低的部署门槛及更长的上下文，在MMLU、CEval等数据集上相比初代有大幅的性能提升。
	// ¥0.006元/千tokens
	ModelChatGLM2_6B_32K = "model_baidu_chatglm2_6b_32k"
	// ModelAquilaChat7B AquilaChat-7B是由智源研究院研发，基于Aquila-7B训练的对话模型，支持流畅的文本对话及多种语言类生成任务，通过定义可扩展的特殊指令规范，实现 AquilaChat对其它模型和工具的调用，且易于扩展
	// ¥0.006元/千tokens
	ModelAquilaChat7B = "model_baidu_aquila_chat7b"
	// ModelBloomz7B BLOOMZ-7B是业内知名的⼤语⾔模型，由BigScience研发并开源，能够以46种语⾔和13种编程语⾔输出⽂本
	// ¥0.006元/千tokens
	ModelBloomz7B = "model_baidu_bloomz_7b"
)

func (ai *BaiduAIImpl) Chat(ctx context.Context, model Model, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req.Fix(model))
	if err != nil {
		return nil, err
	}

	url := ai.modelURL(model)

	resp, err := resty.R().SetQueryParam("access_token", ai.getAccessToken()).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		SetContext(ctx).
		Post(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("chat failed, status code: %d", resp.StatusCode())
	}

	var chatResponse ChatResponse
	if err := json.Unmarshal(resp.Body(), &chatResponse); err != nil {
		return nil, err
	}

	return &chatResponse, nil
}

func (ai *BaiduAIImpl) modelURL(model Model) string {
	var url string
	switch model {
	case ModelErnieBot:
		url = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/completions"
	case ModelErnieBotTurbo:
		url = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/eb-instant"
	case ModelErnieBot4:
		url = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/completions_pro"
	case ModelLlama2_70b:
		url = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/llama_2_70b"
	case ModelLlama2_7b_CN:
		url = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/qianfan_chinese_llama_2_7b"
	case ModelChatGLM2_6B_32K:
		url = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/chatglm2_6b_32k"
	case ModelAquilaChat7B:
		url = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/aquilachat_7b"
	case ModelBloomz7B:
		url = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/bloomz_7b1"
	default:
		panic("invalid model")
	}

	return url
}

func (ai *BaiduAIImpl) ChatStream(ctx context.Context, model Model, req ChatRequest) (<-chan ChatResponse, error) {
	req.Stream = true
	body, err := json.Marshal(req.Fix(model))
	if err != nil {
		return nil, err
	}

	url := ai.modelURL(model)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url+"?access_token="+ai.getAccessToken(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("chat failed, status code: %d", httpResp.StatusCode)
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

				select {
				case <-ctx.Done():
				case res <- ChatResponse{ErrorMessage: fmt.Sprintf("read stream failed: %s", err.Error()), ErrorCode: 10}:
				}
				return
			}

			dataStr := strings.TrimSpace(string(data))
			if dataStr == "" {
				continue
			}

			if !strings.HasPrefix(dataStr, "data:") {
				select {
				case <-ctx.Done():
				case res <- ChatResponse{ErrorMessage: fmt.Sprintf("invalid data: %s", dataStr), ErrorCode: 10}:
				}
				return
			}

			var chatResponse ChatResponse
			if err := json.Unmarshal([]byte(dataStr[5:]), &chatResponse); err != nil {
				select {
				case <-ctx.Done():
				case res <- ChatResponse{ErrorMessage: fmt.Sprintf("unmarshal stream data failed: %v", err), ErrorCode: 10}:
				}
				return
			}

			select {
			case <-ctx.Done():
			case res <- chatResponse:
				if chatResponse.IsEND {
					return
				}
			}
		}
	}()

	return res, nil
}
