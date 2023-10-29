package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/mylxsw/aidea-server/internal/rate"

	"github.com/mylxsw/aidea-server/internal/ai/chat"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	openaiHelper "github.com/mylxsw/aidea-server/internal/ai/openai"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/service"
	"github.com/mylxsw/aidea-server/internal/tencent"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/ternary"
	"github.com/sashabaranov/go-openai"
)

// OpenAIController OpenAI 控制器
type OpenAIController struct {
	conf        *config.Config
	chat        chat.Chat                `autowire:"@"`
	client      *openaiHelper.OpenAI     `autowire:"@"`
	translater  youdao.Translater        `autowire:"@"`
	tencent     *tencent.Tencent         `autowire:"@"`
	messageRepo *repo.MessageRepo        `autowire:"@"`
	securitySrv *service.SecurityService `autowire:"@"`
	userSrv     *service.UserService     `autowire:"@"`
	chatSrv     *service.ChatService     `autowire:"@"`
	limiter     *rate.RateLimiter        `autowire:"@"`

	upgrader websocket.Upgrader
}

// NewOpenAIController 创建 OpenAI 控制器
func NewOpenAIController(resolver infra.Resolver, conf *config.Config) web.Controller {
	ctl := &OpenAIController{conf: conf}
	resolver.MustAutoWire(ctl)

	ctl.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return ctl
}

// Register OpenAIController 路由注册
// 注意：客户端使用了 OpenAI 专用的 SDK，因此这里的路由地址应该与 OpenAI 保持一致，以兼容该 SDK
func (ctl *OpenAIController) Register(router web.Router) {
	// chat 相关接口
	router.Group("/chat", func(router web.Router) {
		router.Any("/completions", ctl.Chat)
	})

	router.Group("/audio", func(router web.Router) {
		router.Post("/transcriptions", ctl.audioTranscriptions)
	})

	// 图像生成相关接口
	router.Group("/images", func(router web.Router) {
		router.Post("/generations", ctl.Images)
	})
}

// audioTranscriptions 语音转文本
func (ctl *OpenAIController) audioTranscriptions(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {

	uploadedFile, err := webCtx.File("file")
	if err != nil {
		log.Errorf("upload file failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	if uploadedFile.Size() > 1024*1024*2 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrFileTooLarge), http.StatusBadRequest)
	}

	tempPath := uploadedFile.GetTempFilename() + "." + uploadedFile.Extension()
	if err := uploadedFile.Store(tempPath); err != nil {
		uploadedFile.Delete()
		log.Errorf("store file failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	defer os.Remove(tempPath)

	log.Debugf("upload file: %s", tempPath)

	model := ternary.If(ctl.conf.UseTencentVoiceToText, "tencent", "whisper-1")

	quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if quota.Quota < quota.Used+coins.GetVoiceCoins(model) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	var resp openai.AudioResponse

	// 使用腾讯语音代替 Whisper
	if ctl.conf.UseTencentVoiceToText {
		res, err := ctl.tencent.VoiceToText(ctx, tempPath)
		if err != nil {
			log.Errorf("tencent voice to text failed: %s", err)
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}

		resp = openai.AudioResponse{Text: res}
	} else {
		req := openai.AudioRequest{
			Model:    model,
			FilePath: tempPath,
		}
		r, err := ctl.client.CreateTranscription(ctx, req)
		if err != nil {
			log.Errorf("createTranscription error: %v", err)
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}

		resp = r
	}

	defer func() {
		if err := quotaRepo.QuotaConsume(ctx, user.ID, coins.GetVoiceCoins(model), repo.NewQuotaUsedMeta("openai-voice", model)); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}
	}()

	return webCtx.JSON(resp)
}

//var openAIChatModelNames = array.Map(array.Filter(openAIModels(), func(m Model, _ int) bool { return m.IsChat }), func(m Model, _ int) string {
//	return strings.TrimPrefix(m.ID, "openai:")
//})

type FinalMessage struct {
	QuotaConsumed int64 `json:"quota_consumed,omitempty"`
	Token         int64 `json:"token,omitempty"`
	QuestionID    int64 `json:"question_id,omitempty"`
	AnswerID      int64 `json:"answer_id,omitempty"`
}

func (m FinalMessage) ToJSON() string {
	data, _ := json.Marshal(m)
	return string(data)
}

func (ctl *OpenAIController) wrapRawResponse(w http.ResponseWriter, cb func()) {
	// 允许跨域
	if ctl.conf.EnableCORS {
		for k, v := range corsHeaders {
			for _, v1 := range v {
				w.Header().Set(k, v1)
			}
		}
	}

	cb()
}

var corsHeaders = http.Header{
	"Access-Control-Allow-Origin":  []string{"*"},
	"Access-Control-Allow-Headers": []string{"*"},
	"Access-Control-Allow-Methods": []string{"GET,POST,OPTIONS,HEAD,PUT,PATCH,DELETE"},
}

type WSError struct {
	Code  int    `json:"code,omitempty"`
	Error string `json:"error,omitempty"`
}

func (e WSError) JSON() []byte {
	data, _ := json.Marshal(e)
	return data
}

// Chat 聊天接口，接口参数参考 https://platform.openai.com/docs/api-reference/chat/create
// 该接口会返回一个 SSE 流，接口参数 stream 总是为 true（忽略客户端设置）
func (ctl *OpenAIController) Chat(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo, w http.ResponseWriter) {
	var req chat.Request
	wsMode := webCtx.Input("ws") == "true"

	var wsConn *websocket.Conn
	if wsMode {
		var err error
		if wsConn, err = ctl.upgrader.Upgrade(w, webCtx.Request().Raw(), corsHeaders); err != nil {
			log.Errorf("upgrade websocket failed: %v", err)
			ctl.wrapRawResponse(w, func() {
				webCtx.JSONError(err.Error(), http.StatusInternalServerError).CreateResponse()
			})

			return
		}
		defer wsConn.Close()

		// 读取第一条消息，用于获取用户输入
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			log.Errorf("read websocket message failed: %v", err)
			wsConn.WriteJSON(WSError{Code: http.StatusInternalServerError, Error: err.Error()})
			return
		}

		if err := json.Unmarshal(msg, &req); err != nil {
			log.Errorf("unmarshal websocket message failed: %v", err)
			wsConn.WriteJSON(WSError{Code: http.StatusBadRequest, Error: err.Error()})
			return
		}

		req.WebSocket = true
	} else {
		if err := webCtx.Unmarshal(&req); err != nil {
			ctl.wrapRawResponse(w, func() {
				webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest).CreateResponse()
			})
			return
		}
	}

	// 查询 room 信息，修正最大上下文消息数量
	var maxContextLength int64 = 5
	if req.RoomID > 0 {
		func() {
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			room, err := ctl.chatSrv.Room(ctx, user.ID, req.RoomID)
			if err != nil {
				log.With(req).Errorf("get room info failed: %s", err)
			}

			if room.MaxContext > 0 {
				maxContextLength = room.MaxContext
			}
		}()
	}

	fixRes, err := req.Fix(ctl.chat, maxContextLength)
	if err != nil {
		if req.WebSocket {
			wsConn.WriteJSON(WSError{Code: http.StatusBadRequest, Error: err.Error()})
		} else {
			ctl.wrapRawResponse(w, func() {
				webCtx.JSONError(err.Error(), http.StatusBadRequest).CreateResponse()
			})
		}

		return
	}

	req = fixRes.Request

	// 基于模型的流控，避免单一模型用户过度使用
	if ctl.conf.EnableModelRateLimit {
		if err := ctl.limiter.Allow(ctx, fmt.Sprintf("chat-limit:u:%d:m:%s:minute", user.ID, req.Model), redis_rate.PerMinute(5)); err != nil {
			if errors.Is(err, rate.ErrRateLimitExceeded) {
				if req.WebSocket {
					wsConn.WriteJSON(WSError{Code: http.StatusBadRequest, Error: common.Text(webCtx, ctl.translater, "操作频率过高，请稍后再试")})
				} else {
					ctl.wrapRawResponse(w, func() {
						webCtx.JSONError(common.Text(webCtx, ctl.translater, "操作频率过高，请稍后再试"), http.StatusBadRequest).CreateResponse()
					})
				}
				return
			}

			log.WithFields(log.Fields{"user_id": user.ID, "req": req}).Errorf("check rate limit failed: %s", err)
		}
	}

	// 免费模型
	leftCount, maxFreeCount := ctl.userSrv.FreeChatRequestCounts(ctx, user.ID, req.Model)
	isFreeRequest := leftCount > 0

	if !isFreeRequest {
		quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
		if err != nil {
			log.Errorf("get user quota failed: %s", err)
			if req.WebSocket {
				wsConn.WriteJSON(WSError{Code: http.StatusInternalServerError, Error: common.Text(webCtx, ctl.translater, common.ErrInternalError)})
			} else {
				ctl.wrapRawResponse(w, func() {
					webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError).CreateResponse()
				})
			}
			return
		}

		// 获取当前用户剩余的智慧果数量，如果不足，则返回错误
		// 假设当前响应消耗 2 个智慧果
		restQuota := quota.Quota - quota.Used - coins.GetOpenAITextCoins(req.ResolveCalFeeModel(ctl.conf), int64(fixRes.InputTokens)) - 2
		if restQuota < 0 {
			if maxFreeCount > 0 {
				if req.WebSocket {
					wsConn.WriteJSON(WSError{Code: http.StatusPaymentRequired, Error: common.Text(webCtx, ctl.translater, "今日免费额度已不足，请充值后再试")})
				} else {
					ctl.wrapRawResponse(w, func() {
						webCtx.JSONError(common.Text(webCtx, ctl.translater, "今日免费额度已不足，请充值后再试"), http.StatusPaymentRequired).CreateResponse()
					})
				}
				return
			}

			if req.WebSocket {
				wsConn.WriteJSON(WSError{Code: http.StatusPaymentRequired, Error: common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough)})
			} else {
				ctl.wrapRawResponse(w, func() {
					webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired).CreateResponse()
				})
			}
			return
		}
	}

	// 内容安全检测
	shouldReturn := ctl.securityCheck(webCtx, req, user, w, wsConn)
	if shouldReturn {
		return
	}

	// 写入用户消息
	var questionID int64
	if ctl.conf.EnableRecordChat {
		qid, err := ctl.messageRepo.Add(ctx, repo.MessageAddReq{
			UserID:  user.ID,
			Message: req.Messages[len(req.Messages)-1].Content,
			Role:    repo.MessageRoleUser,
			RoomID:  req.RoomID,
			Model:   req.Model,
			Status:  repo.MessageStatusSucceed,
		})
		if err != nil {
			log.With(req).Errorf("add message failed: %s", err)
		}

		questionID = qid
	}

	// log.WithFields(log.Fields{"req": req}).Debugf("chat request")

	var replyText string

	chatCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	stream, err := ctl.chat.ChatStream(chatCtx, req)
	if err != nil {
		log.Errorf("聊天请求失败，模型 %s: %v", req.Model, err)
		if req.WebSocket {
			wsConn.WriteJSON(WSError{Code: http.StatusInternalServerError, Error: common.Text(webCtx, ctl.translater, common.ErrInternalError)})
		} else {
			ctl.wrapRawResponse(w, func() {
				webCtx.JSONError(err.Error(), http.StatusInternalServerError).CreateResponse()
			})
		}
		return
	}

	if !req.WebSocket {
		ctl.wrapRawResponse(w, func() {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
		})
	}

	defer func() {
		var chatErrorMessage string
		if chatError := recover(); chatError != nil {
			chatErrorMessage = fmt.Sprintf("%v", chatError)
		}

		replyText = strings.TrimSpace(replyText)
		messages := append(req.Messages, chat.Message{
			Role:    "assistant",
			Content: replyText,
		})

		realWordCount, _ := chat.MessageTokenCount(messages, req.Model)
		quotaConsumed := coins.GetOpenAITextCoins(req.ResolveCalFeeModel(ctl.conf), int64(realWordCount))

		// 返回自定义控制信息，告诉客户端当前消耗情况
		if isFreeRequest || replyText == "" {
			// 免费请求，不扣除智慧果
			quotaConsumed = 0
		}

		// 响应内容为空，报错给客户端
		if replyText == "" && chatErrorMessage == "" {
			chatErrorMessage = "响应内容为空"
		}

		if chatErrorMessage != "" {
			log.F(log.M{"req": req, "user": user}).Errorf("聊天失败：%s", chatErrorMessage)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var answerID int64
		if ctl.conf.EnableRecordChat {
			// 写入用户消息
			answerID, err = ctl.messageRepo.Add(ctx, repo.MessageAddReq{
				UserID:        user.ID,
				Message:       replyText,
				Role:          repo.MessageRoleAssistant,
				QuotaConsumed: quotaConsumed,
				TokenConsumed: int64(realWordCount),
				RoomID:        req.RoomID,
				Model:         req.Model,
				PID:           questionID,
				Status:        int64(ternary.If(chatErrorMessage != "", repo.MessageStatusFailed, repo.MessageStatusSucceed)),
				Error:         chatErrorMessage,
			})
			if err != nil {
				log.With(req).Errorf("add message failed: %s", err)
			}
		}

		finalMsg := FinalMessage{QuestionID: questionID, AnswerID: answerID}
		if user.InternalUser() {
			finalMsg.QuotaConsumed = quotaConsumed
			finalMsg.Token = int64(realWordCount)
		}

		// final 消息为定制消息，用于告诉 AIdea 客户端当前回话的资源消耗情况以及服务端信息
		finalWord := openai.ChatCompletionStreamResponse{
			ID: "final",
			Choices: []openai.ChatCompletionStreamChoice{
				{
					Index:        0,
					FinishReason: "",
					Delta: openai.ChatCompletionStreamChoiceDelta{
						Content: finalMsg.ToJSON(),
						Role:    "system",
					},
				},
			},
			Model: req.Model,
		}

		if req.WebSocket {
			if chatErrorMessage == "" || replyText != "" {
				if err := wsConn.WriteJSON(finalWord); err != nil {
					log.Warningf("write response failed: %v", err)
				}
			} else {
				if err := wsConn.WriteJSON(WSError{Code: http.StatusInternalServerError, Error: chatErrorMessage}); err != nil {
					log.Warningf("write response failed: %v", err)
				}
			}
		} else {
			data, _ := json.Marshal(finalWord)
			if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
				log.Warningf("write response failed: %v", err)
			}

			// 写入结束标志
			_, _ = w.Write([]byte("data: [DONE]\n\n"))

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		// 更新用户免费聊天次数
		if chatErrorMessage == "" || replyText != "" {
			if err := ctl.userSrv.UpdateFreeChatCount(ctx, user.ID, req.Model); err != nil {
				log.WithFields(log.Fields{
					"user_id": user.ID,
					"model":   req.Model,
				}).Errorf("update free chat count failed: %s", err)
			}
		}

		// 扣除智慧果
		if !isFreeRequest && quotaConsumed > 0 {
			if err := quotaRepo.QuotaConsume(ctx, user.ID, quotaConsumed, repo.NewQuotaUsedMeta("chat", req.Model)); err != nil {
				log.Errorf("used quota add failed: %s", err)
			}
		}
	}()

	// 生成 SSE 流
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	id := 0
	for {
		timer.Reset(15 * time.Second)

		select {
		case <-timer.C:
			panic("两次响应之间等待时间过长，强制中断")
		case <-ctx.Done():
			return
		case res, ok := <-stream:
			if !ok {
				return
			}

			id++

			if res.ErrorCode != "" {
				log.With(req).Errorf("chat error: %v", res)
				return
			}

			resp := openai.ChatCompletionStreamResponse{
				ID:      strconv.Itoa(id),
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []openai.ChatCompletionStreamChoice{
					{
						Delta: openai.ChatCompletionStreamChoiceDelta{
							Role:    "assistant",
							Content: res.Text,
						},
					},
				},
			}

			replyText += res.Text

			if req.WebSocket {
				err := wsConn.WriteJSON(resp)
				if err != nil {
					log.Errorf("write response failed: %v", err)
					return
				}
			} else {
				data, err := json.Marshal(resp)
				if err != nil {
					log.Errorf("marshal response failed: %v", err)
					return
				}

				if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
					log.Errorf("write response failed: %v", err)
					return
				}

				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
	}
}

// 内容安全检测
func (ctl *OpenAIController) securityCheck(webCtx web.Context, req chat.Request, user *auth.User, w http.ResponseWriter, wsConn *websocket.Conn) bool {
	// content := strings.Join(array.Map(req.Messages, func(msg openai.ChatCompletionMessage, _ int) string { return msg.Content }), "\n")
	content := req.Messages[len(req.Messages)-1].Content
	if checkRes := ctl.securitySrv.ChatDetect(content); checkRes != nil {
		if !checkRes.Safe {
			log.WithFields(log.Fields{
				"user_id": user.ID,
				"details": checkRes,
				"content": content,
			}).Warningf("用户 %d 违规，违规内容：%s", user.ID, checkRes.Reason)

			if req.WebSocket {
				if err := wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(
					`{"id":"chatxxx1","object":"chat.completion.chunk","created":%d,"model":"gpt-3.5-turbo-0613","choices":[{"index":0,"delta":{"role":"assistant","content":%s},"finish_reason":null}]}`+"\n\n",
					time.Now().Unix(),
					strconv.Quote("内容违规，已被系统拦截，如有疑问邮件联系：support@aicode.cc"),
				))); err != nil {
					log.Warningf("write response failed: %v", err)
				}

			} else {
				ctl.wrapRawResponse(w, func() {
					w.Header().Set("Content-Type", "text/event-stream")
					w.Header().Set("Cache-Control", "no-cache")
					w.Header().Set("Connection", "keep-alive")
				})

				w.Write([]byte(fmt.Sprintf(
					`data: {"id":"chatxxx1","object":"chat.completion.chunk","created":%d,"model":"gpt-3.5-turbo-0613","choices":[{"index":0,"delta":{"role":"assistant","content":%s},"finish_reason":null}]}`+"\n\n",
					time.Now().Unix(),
					strconv.Quote("内容违规，已被系统拦截，如有疑问邮件联系：support@aicode.cc"),
				)))

				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}

				w.Write([]byte(fmt.Sprintf(
					`data: {"id":"chatxxx2","object":"chat.completion.chunk","created":%d,"model":"gpt-3.5-turbo-0613","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\n",
					time.Now().Unix(),
				)))

				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}

				w.Write([]byte("data: [DONE]\n\n"))

				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}

			return true
		}
	}

	return false
}

// Images 图像生成接口，接口参数参考 https://platform.openai.com/docs/api-reference/images/create
func (ctl *OpenAIController) Images(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {
	quota, err := quotaRepo.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if quota.Quota < quota.Used+int64(coins.GetUnifiedImageGenCoins()) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	var req openai.ImageRequest
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	resp, err := ctl.client.CreateImage(ctx, req)
	if err != nil {
		log.Errorf("createImage error: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	defer func() {
		if err := quotaRepo.QuotaConsume(ctx, user.ID, int64(coins.GetUnifiedImageGenCoins()), repo.NewQuotaUsedMeta("openai-image", "DALL·E")); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}
	}()

	return webCtx.JSON(resp)
}
