package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/ai/control"
	openaiHelper "github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/ai/streamwriter"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/rate"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/pkg/tencent"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"github.com/mylxsw/go-utils/array"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/go-redis/redis_rate/v10"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers/common"
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
	client      openaiHelper.Client      `autowire:"@"`
	translater  youdao.Translater        `autowire:"@"`
	tencent     *tencent.Tencent         `autowire:"@"`
	messageRepo *repo.MessageRepo        `autowire:"@"`
	securitySrv *service.SecurityService `autowire:"@"`
	userSrv     *service.UserService     `autowire:"@"`
	chatSrv     *service.ChatService     `autowire:"@"`
	limiter     *rate.RateLimiter        `autowire:"@"`
	repo        *repo.Repository         `autowire:"@"`

	upgrader websocket.Upgrader

	apiMode bool // 是否为 OpenAI API 模式
}

// NewOpenAIController 创建 OpenAI 控制器
func NewOpenAIController(resolver infra.Resolver, conf *config.Config, apiMode bool) web.Controller {
	ctl := &OpenAIController{conf: conf, apiMode: apiMode}
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
// https://platform.openai.com/docs/api-reference/audio/createTranscription
func (ctl *OpenAIController) audioTranscriptions(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {
	// TODO 增加客户端控制语音转文本的参数：model/file/language/prompt/response_format/temperature
	model := ternary.If(ctl.conf.UseTencentVoiceToText, "tencent", "whisper-1")

	if ctl.conf.EnableModelRateLimit {
		if err := ctl.limiter.Allow(ctx, fmt.Sprintf("chat-limit:u:%d:m:%s:minute", user.ID, model), redis_rate.PerMinute(5)); err != nil {
			if errors.Is(err, rate.ErrRateLimitExceeded) {
				return webCtx.JSONError("操作频率过高，请稍后再试", http.StatusTooManyRequests)
			}

			log.F(log.M{"user_id": user.ID}).Errorf("check rate limit failed: %s", err)
		}
	}

	quota, err := ctl.userSrv.UserQuota(ctx, user.ID)
	if err != nil {
		log.F(log.M{"user_id": user.ID}).Errorf("查询用户智慧果余量失败: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	needCoins := coins.GetVoiceCoins(model)
	if quota.Rest-quota.Freezed < needCoins {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	// 冻结本次所需要的智慧果
	if err := ctl.userSrv.FreezeUserQuota(ctx, user.ID, needCoins); err != nil {
		log.F(log.M{"user_id": user.ID, "quota": needCoins}).Errorf("freeze user quota failed: %s", err)
	} else {
		defer func(ctx context.Context) {
			// 解冻智慧果
			if err := ctl.userSrv.UnfreezeUserQuota(ctx, user.ID, needCoins); err != nil {
				log.F(log.M{"user_id": user.ID, "quota": needCoins}).Errorf("unfreeze user quota failed: %s", err)
			}
		}(ctx)
	}

	uploadedFile, err := webCtx.File("file")
	if err != nil {
		log.Errorf("upload file failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	if uploadedFile.Size() > 1024*1024*10 {
		log.F(log.M{"file": uploadedFile.GetTempFilename(), "size": float64(uploadedFile.Size()) / 1024.0 / 1024.0}).Errorf("uploaded video file too large")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrFileTooLarge), http.StatusBadRequest)
	}

	var tempPath string
	if uploadedFile.Size() >= 2*1024*1024 {
		if err := misc.WavToMp3(uploadedFile.SavePath, uploadedFile.GetTempFilename()+".mp3"); err != nil {
			log.F(log.M{"size": float64(uploadedFile.Size()) / 1024.0 / 1024.0}).Warningf("convert wav to mp3 failed: %s", err)
		} else {
			misc.NoError(os.Remove(uploadedFile.GetTempFilename()))
			tempPath = uploadedFile.GetTempFilename() + ".mp3"

			log.F(log.M{
				"size":    float64(uploadedFile.Size()) / 1024.0 / 1024.0,
				"file":    tempPath,
				"resized": float64(misc.FileSize(tempPath)) / 1024.0 / 1024.0,
			}).Debug("convert m4a to mp3 file")
		}
	}

	if !strings.HasSuffix(tempPath, ".mp3") {
		tempPath = uploadedFile.GetTempFilename() + "." + uploadedFile.Extension()
		if err := uploadedFile.Store(tempPath); err != nil {
			misc.NoError(uploadedFile.Delete())
			log.Errorf("store file failed: %s", err)
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}
	}

	defer func() { misc.NoError(os.Remove(tempPath)) }()

	log.F(log.M{
		"size": float64(uploadedFile.Size()) / 1024.0 / 1024.0,
	}).Debugf("upload file: %s", tempPath)

	var resp openai.AudioResponse

	// 使用腾讯语音代替 Whisper
	if ctl.conf.UseTencentVoiceToText {
		res, err := ctl.tencent.VoiceToText(ctx, tempPath)
		if err != nil {
			log.Errorf("tencent voice to text failed: %s", err)
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}

		log.F(log.M{"text": res, "file": tempPath, "size": float64(uploadedFile.Size()) / 1024.0 / 1024.0}).Debugf("tencent voice to text success")

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

type FinalMessage struct {
	Type          string `json:"type,omitempty"`
	QuotaConsumed int64  `json:"quota_consumed,omitempty"`
	Token         int64  `json:"token,omitempty"`
	QuestionID    int64  `json:"question_id,omitempty"`
	AnswerID      int64  `json:"answer_id,omitempty"`
	Info          string `json:"info,omitempty"`
	Error         string `json:"error,omitempty"`
}

func (m FinalMessage) ToJSON() string {
	data, _ := json.Marshal(m)
	return string(data)
}

// Chat 聊天接口，接口参数参考 https://platform.openai.com/docs/api-reference/chat/create
// 该接口会返回一个 SSE 流，接口参数 stream 总是为 true（忽略客户端设置）
func (ctl *OpenAIController) Chat(ctx context.Context, webCtx web.Context, user *auth.UserOptional, quotaRepo *repo.QuotaRepo, w http.ResponseWriter, client *auth.ClientInfo) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if user.User == nil && ctl.conf.FreeChatEnabled && client.IsIOS() {
		// 匿名用户访问
		user.User = &auth.User{
			ID:   0,
			Name: "anonymous",
		}
	}

	if user.User == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "用户未登录，请先登录后再试"}`))
		return
	}

	// 流控，避免单一用户过度使用
	if err := ctl.rateLimitPass(ctx, client, user.User); err != nil {
		if errors.Is(err, rate.ErrDailyFreeLimitExceeded) {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error": %s}`, strconv.Quote(err.Error()))))
		return
	}

	sw, req, err := streamwriter.New[chat.Request](
		webCtx.Input("ws") == "true", ctl.conf.EnableCORS, webCtx.Request().Raw(), w,
	)
	if err != nil {
		log.F(log.M{"user": user.User.ID, "client": client}).Errorf("create stream writer failed: %s", err)
		return
	}
	defer sw.Close()

	subCtx, subCancel := context.WithCancel(ctx)
	sw.SetOnClosed(subCancel)

	// 匿名用户，使用免费模型代替
	if user.User.ID == 0 && ctl.conf.FreeChatModel != "" {
		req.Model = ctl.conf.FreeChatModel
	}

	// 请求参数预处理
	var inputTokenCount, maxContextLen int64

	if ctl.apiMode {
		// API 模式下，还原 n 参数原始值（不支持 room 上下文配置）
		req.N = int(req.RoomID)
		icnt, err := chat.MessageTokenCount(req.Messages, req.Model)
		if err != nil {
			misc.NoError(sw.WriteErrorStream(err, http.StatusBadRequest))
			return
		}

		inputTokenCount = int64(icnt)
	} else {
		// 每次对话用户可以手动选择要使用的模型
		selectedModel, chatMessages, err := ctl.resolveModelMessages(subCtx, req.Messages, user, req.TempModel)
		if err != nil {
			selectedModel, chatMessages, err = ctl.resolveModelMessages(subCtx, req.Messages, user, req.Model)
			if err != nil {
				misc.NoError(sw.WriteErrorStream(err, http.StatusBadRequest))
				return
			}
		}

		req.Model = selectedModel
		req.Messages = chatMessages

		// 模型最大上下文长度限制
		maxContextLen = ctl.loadRoomContextLen(subCtx, req.RoomID, user.User.ID)
		req, inputTokenCount, err = req.Fix(ctl.chat, maxContextLen, ternary.If(user.User.ID > 0, 1000*200, 1000))
		if err != nil {
			misc.NoError(sw.WriteErrorStream(err, http.StatusBadRequest))
			return
		}
	}

	// 检查请求参数
	// 上下文消息为空（含当前消息）
	if len(req.Messages) == 0 {
		misc.NoError(sw.WriteErrorStream(errors.New(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest)), http.StatusBadRequest))
		return
	}

	// 免费模型
	// 获取当前用户剩余的智慧果数量，如果不足，则返回错误
	var leftCount, maxFreeCount int
	if user.User.ID > 0 {
		leftCount, maxFreeCount = ctl.chatSrv.FreeChatRequestCounts(subCtx, user.User.ID, req.Model)
	} else {
		// 匿名用户，每次都是免费的，不限制次数，通过流控来限制访问
		leftCount, maxFreeCount = 1, 0
	}

	// 查询模型信息
	mod := ctl.chatSrv.Model(subCtx, req.Model)
	if mod == nil || mod.Status == repo.ModelStatusDisabled {
		misc.NoError(sw.WriteErrorStream(errors.New("当前模型暂不可用，请选择其它模型"), http.StatusNotFound))
		return
	}

	if leftCount <= 0 {
		quota, needCoins, err := ctl.queryChatQuota(subCtx, user.User, sw, webCtx, inputTokenCount, mod)
		if err != nil {
			return
		}

		// 智慧果不足
		if quota.Rest-quota.Freezed < needCoins {
			if maxFreeCount > 0 {
				misc.NoError(sw.WriteErrorStream(errors.New(common.Text(webCtx, ctl.translater, "今日免费额度已不足，请充值后再试")), http.StatusPaymentRequired))
				return
			}

			misc.NoError(sw.WriteErrorStream(errors.New(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough)), http.StatusPaymentRequired))
			return
		}

		// 冻结本次所需要的智慧果
		if err := ctl.userSrv.FreezeUserQuota(ctx, user.User.ID, needCoins); err != nil {
			log.F(log.M{"user_id": user.User.ID, "quota": needCoins}).Errorf("freeze user quota failed: %s", err)
		} else {
			defer func(ctx context.Context) {
				// 解冻智慧果
				if err := ctl.userSrv.UnfreezeUserQuota(ctx, user.User.ID, needCoins); err != nil {
					log.F(log.M{"user_id": user.User.ID, "quota": needCoins}).Errorf("unfreeze user quota failed: %s", err)
				}
			}(ctx)
		}
	}

	// 内容安全检测
	if err := ctl.contentSafety(req, user.User, sw); err != nil {
		return
	}

	var quotaConsume QuotaConsume

	startTime := time.Now()
	defer func() {
		log.F(log.M{
			"user_id": user.User.ID,
			"client":  client,
			"room_id": req.RoomID,
			"elapse":  time.Since(startTime).Seconds(),
		}).
			Infof(
				"接收到聊天请求，模型 %s, 上下文消息数量 %d, 输入 token 数量 %d，输出 token 数量 %d，消耗智慧果 %d",
				req.Model,
				len(req.Messages),
				ternary.If(quotaConsume.InputTokens > int(inputTokenCount), quotaConsume.InputTokens, int(inputTokenCount)),
				quotaConsume.OutputTokens,
				quotaConsume.TotalPrice,
			)
	}()

	// 写入用户消息
	questionID := ctl.saveChatQuestion(subCtx, user.User, req)

	maxRetryTimes := 1
	if cq, ok := ctl.chat.(chat.ChannelQuery); ok {
		maxRetryTimes = len(cq.Channels(req.Model))
	}

	replyText, err, done := ctl.chatWithRetry(subCtx, req, user, sw, webCtx, questionID, startTime, 0, maxRetryTimes)
	if done {
		return
	}

	chatErrorMessage := ternary.IfLazy(err == nil, func() string { return "" }, func() string { return err.Error() })
	if chatErrorMessage != "" {
		log.F(log.M{"req": req, "user_id": user.User.ID, "reply": replyText, "elapse": time.Since(startTime).Seconds()}).
			Errorf("chat failed, model: %s, error: %s", req.Model, chatErrorMessage)
	}

	// 返回自定义控制信息，告诉客户端当前消耗情况
	quotaConsume = ctl.resolveConsumeQuota(req, replyText, leftCount > 0, mod)

	func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// 写入用户消息
		answerID := ctl.saveChatAnswer(ctx, user.User, replyText, quotaConsume.TotalPrice, quotaConsume.TotalTokens(), req, questionID, chatErrorMessage)

		if errors.Is(ErrChatResponseEmpty, err) {
			misc.NoError(sw.WriteErrorStream(err, http.StatusInternalServerError))
		} else {
			if !ctl.apiMode {
				// final 消息为定制消息，用于告诉 AIdea 客户端当前的资源消耗情况以及服务端信息
				finalWord := ctl.buildFinalSystemMessage(questionID, answerID, user.User, quotaConsume.TotalPrice, quotaConsume.TotalTokens(), req, maxContextLen, chatErrorMessage)
				misc.NoError(sw.WriteStream(finalWord))
			}
		}
	}()

	// 更新用户免费聊天次数
	if replyText != "" {
		func() {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := ctl.chatSrv.UpdateFreeChatCount(ctx, user.User.ID, req.Model); err != nil {
				log.WithFields(log.Fields{
					"user_id": user.User.ID,
					"model":   req.Model,
				}).Errorf("update free chat count failed: %s", err)
			}
		}()
	}

	// 扣除智慧果
	if leftCount <= 0 && quotaConsume.TotalPrice > 0 {
		func() {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			meta := repo.NewQuotaUsedMeta("chat", req.Model)
			meta.InputToken = quotaConsume.InputTokens
			meta.OutputToken = quotaConsume.OutputTokens
			meta.InputPrice = quotaConsume.InputPrice
			meta.OutputPrice = quotaConsume.OutputPrice
			meta.ReqPrice = quotaConsume.PerReqPrice

			if err := quotaRepo.QuotaConsume(ctx, user.User.ID, quotaConsume.TotalPrice, meta); err != nil {
				log.Errorf("used quota add failed: %s", err)
			}
		}()
	}
}

func (ctl *OpenAIController) resolveModelMessages(ctx context.Context, messages chat.Messages, user *auth.UserOptional, model string) (string, chat.Messages, error) {
	if model == "" {
		return "", nil, errors.New("model is required")
	}

	// 支持 V2 版本的 homeModel 请求
	// model 格式为 v2@{type}|{id}
	if strings.HasPrefix(model, "v2@") {
		models := array.ToMap(ctl.chatSrv.Models(ctx, true), func(item repo.Model, _ int) string {
			return item.ModelId
		})

		homeModel, err := ctl.userSrv.QueryHomeModel(ctx, models, user.User.ID, strings.TrimPrefix(model, "v2@"))
		if err != nil {
			return "", nil, err
		}

		if strings.TrimSpace(homeModel.Prompt) != "" {
			contextMessages := array.Filter(messages, func(item chat.Message, _ int) bool { return item.Role != "system" })
			messages = append(chat.Messages{{Role: "system", Content: homeModel.Prompt}}, contextMessages...)
		}

		return homeModel.ModelID, messages, nil
	}

	return model, messages, nil
}

func (ctl *OpenAIController) chatWithRetry(
	ctx context.Context,
	req *chat.Request,
	user *auth.UserOptional,
	sw *streamwriter.StreamWriter,
	webCtx web.Context,
	questionID int64,
	startTime time.Time,
	retryTimes int,
	maxRetryTimes int,
) (string, error, bool) {
	// 发起聊天请求并返回 SSE/WS 流
	replyText, err := ctl.handleChat(ctx, req, user.User, sw, webCtx, questionID, retryTimes)
	if errors.Is(err, ErrChatResponseHasSent) {
		return "", nil, true
	}

	// 以下两种情况再次尝试
	// 1. 聊天响应为空
	// 2. 两次响应之间等待时间过长，强制中断，同时响应为空
	if errors.Is(err, ErrChatResponseEmpty) || (errors.Is(err, ErrChatResponseGapTimeout) && replyText == "") {
		// 如果用户等待时间超过 60s，则不再重试，避免用户等待时间过长
		if startTime.Add(60 * time.Second).After(time.Now()) {
			// 重试次数超过最大重试次数
			if retryTimes >= maxRetryTimes {
				log.F(log.M{"req": req, "user_id": user.User.ID}).Errorf("response is empty, model: %s, retry times exceed the limit", req.Model)
				return "", err, true
			}

			log.F(log.M{"req": req, "user_id": user.User.ID}).Warningf("response is empty, try requesting again(%d), model: %s", retryTimes+1, req.Model)
			return ctl.chatWithRetry(ctx, req, user, sw, webCtx, questionID, startTime, retryTimes+1, maxRetryTimes)
		}
	}
	return replyText, err, false
}

func (ctl *OpenAIController) handleChat(
	ctx context.Context,
	req *chat.Request,
	user *auth.User,
	sw *streamwriter.StreamWriter,
	webCtx web.Context,
	questionID int64,
	retryTimes int,
) (string, error) {
	chatCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	// 如果是重试请求，则优先使用备用模型
	if retryTimes > 0 {
		chatCtx = control.NewContext(chatCtx, &control.Control{PreferBackup: true, RetryTimes: retryTimes})
	}

	newReq := req.Clone()

	stream, err := ctl.chat.ChatStream(chatCtx, newReq.Purification())
	if err != nil {
		// 更新问题为失败状态
		ctl.makeChatQuestionFailed(ctx, questionID, err)

		// 内容违反内容安全策略
		if errors.Is(err, chat.ErrContentFilter) {
			ctl.sendViolateContentPolicyResp(sw, "")
			return "", ErrChatResponseHasSent
		}

		log.WithFields(log.Fields{"user_id": user.ID, "retry_times": retryTimes}).Errorf("chat request failed, model %s: %v", req.Model, err)

		misc.NoError(sw.WriteErrorStream(errors.New(common.Text(webCtx, ctl.translater, common.ErrInternalError)), http.StatusInternalServerError))
		return "", ErrChatResponseHasSent
	}

	replyText, err := ctl.writeChatResponse(chatCtx, req, stream, user, sw)
	if err != nil {
		return replyText, err
	}

	replyText = strings.TrimSpace(replyText)

	if replyText == "" {
		return replyText, ErrChatResponseEmpty
	}

	return replyText, nil
}

var (
	ErrChatResponseEmpty      = errors.New("response is empty")
	ErrChatResponseHasSent    = errors.New("response has sent")
	ErrChatResponseGapTimeout = errors.New("force close after too long of inactivity between responses")
)

func (ctl *OpenAIController) writeChatResponse(ctx context.Context, req *chat.Request, stream <-chan chat.Response, user *auth.User, sw *streamwriter.StreamWriter) (string, error) {
	var replyText string

	// 生成 SSE 流
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()

	id := 0
	for {
		if id > 0 {
			timer.Reset(30 * time.Second)
		}

		select {
		case <-timer.C:
			return replyText, ErrChatResponseGapTimeout
		case <-ctx.Done():
			return replyText, nil
		case res, ok := <-stream:
			if !ok {
				return replyText, nil
			}

			id++

			if res.ErrorCode != "" {
				if id <= 1 {
					log.WithFields(log.Fields{"req": req, "user_id": user.ID}).Warningf("chat response failed, we need a retry: %v", res)
					return replyText, ErrChatResponseEmpty
				}

				log.WithFields(log.Fields{"req": req, "user_id": user.ID}).Errorf("chat response failed: %v", res)

				if res.Error != "" {
					res.Text = fmt.Sprintf("\n\n---\nSorry, we encountered some errors. Here are the error details: \n```\n%s\n```\n", res.Error)
				} else {
					return replyText, nil
				}
			} else {
				replyText += res.Text
			}

			resp := ChatCompletionStreamResponse{
				ID:      strconv.Itoa(id),
				Created: time.Now().Unix(),
				Model:   req.Model,
				Object:  "chat.completion",
				Choices: []ChatCompletionStreamChoice{
					{
						Delta: ChatCompletionStreamChoiceDelta{
							Role:    "assistant",
							Content: res.Text,
						},
					},
				},
			}

			if err := sw.WriteStream(resp); err != nil {
				log.F(log.M{"req": req, "user_id": user.ID}).Warningf("write response failed: %v", err)
				return replyText, nil
			}
		}
	}
}

type ChatCompletionStreamResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []ChatCompletionStreamChoice `json:"choices"`
}

type ChatCompletionStreamChoice struct {
	Index        int                             `json:"index"`
	Delta        ChatCompletionStreamChoiceDelta `json:"delta"`
	FinishReason *string                         `json:"finish_reason,omitempty"`
}

type ChatCompletionStreamChoiceDelta struct {
	Content      string               `json:"content"`
	Role         string               `json:"role,omitempty"`
	FunctionCall *openai.FunctionCall `json:"function_call,omitempty"`
}

// buildFinalSystemMessage 构建最后一条消息，该消息为系统消息，用于告诉 AIdea 客户端当前的资源消耗情况以及服务端信息
func (*OpenAIController) buildFinalSystemMessage(
	questionID int64,
	answerID int64,
	user *auth.User,
	quotaConsumed int64,
	realTokenConsumed int,
	req *chat.Request,
	maxContextLen int64,
	chatErrorMessage string,
) ChatCompletionStreamResponse {
	finalMsg := FinalMessage{
		Type:       "summary",
		QuestionID: questionID,
		AnswerID:   answerID,
		Token:      int64(realTokenConsumed),
		Error:      chatErrorMessage,
	}

	if len(req.Messages) >= int(maxContextLen*3)-1 || realTokenConsumed > 2000 {
		if req.RoomID <= 1 {
			finalMsg.Info = fmt.Sprintf("本次请求消耗了 %d 个 Token。\n\nAI 记住的对话信息越多，消耗的 Token 和智慧果也越多。\n\n如果新问题和之前的对话无关，请创建新对话。", realTokenConsumed)
		} else {
			finalMsg.Info = fmt.Sprintf("本次请求消耗了 %d 个 Token。\n\nAI 记住的对话信息越多，消耗的 Token 和智慧果也越多。\n\n如果新问题和之前的对话无关，请使用“[新对话](aidea-command://reset-context)”来重置对话上下文。", realTokenConsumed)
		}
	}

	if user.InternalUser() {
		finalMsg.QuotaConsumed = quotaConsumed
	}

	return ChatCompletionStreamResponse{
		ID:      "final",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Choices: []ChatCompletionStreamChoice{
			{
				Index: 0,
				Delta: ChatCompletionStreamChoiceDelta{
					Content: finalMsg.ToJSON(),
					Role:    "system",
				},
			},
		},
		Model: req.Model,
	}
}

// queryChatQuota 检查用户智慧果余量是否足够
func (ctl *OpenAIController) queryChatQuota(
	ctx context.Context,
	user *auth.User,
	sw *streamwriter.StreamWriter,
	webCtx web.Context,
	inputTokenCount int64,
	mod *repo.Model,
) (quota *service.UserQuota, needCoins int64, err error) {
	quota, err = ctl.userSrv.UserQuota(ctx, user.ID)
	if err != nil {
		log.F(log.M{"user_id": user.ID}).Errorf("查询用户智慧果余量失败: %s", err)
		misc.NoError(sw.WriteErrorStream(errors.New(common.Text(webCtx, ctl.translater, common.ErrInternalError)), http.StatusInternalServerError))

		return nil, 0, err
	}

	// 假设本次请求将会消耗 500 个输出 Token
	return quota, coins.GetTextModelCoins(mod.ToCoinModel(), inputTokenCount, 500), nil
}

func (ctl *OpenAIController) rateLimitPass(ctx context.Context, client *auth.ClientInfo, user *auth.User) error {
	if ctl.conf.EnableModelRateLimit {
		if err := ctl.limiter.Allow(ctx, fmt.Sprintf("chat-limit:u:%d:minute", user.ID), redis_rate.PerMinute(10)); err != nil {
			if errors.Is(err, rate.ErrRateLimitExceeded) {
				return rate.ErrRateLimitExceeded
			}

			log.F(log.M{"user_id": user.ID}).Errorf("聊天请求频率过高： %s", err)
		}
	}

	// 匿名用户每日免费次数限制
	if ctl.conf.FreeChatEnabled && user.ID == 0 {
		lim := redis_rate.Limit{Rate: ctl.conf.FreeChatDailyLimit, Burst: ctl.conf.FreeChatDailyLimit, Period: time.Hour * 24}
		if err := ctl.limiter.Allow(ctx, fmt.Sprintf("chat-limit:anonymous:%s:daily", client.IP), lim); err != nil {
			log.F(log.M{"ip": client.IP}).Errorf("今日免费次数已用完（IP）: %s", err)
			return rate.ErrDailyFreeLimitExceeded
		}

		// 全局限制免费次数，这里是总次数，不区分用户
		if ctl.conf.FreeChatDailyGlobalLimit > 0 {
			dailyGlobalLimitKey := fmt.Sprintf("chat-limit:free:daily:%s", time.Now().Format("2006-01-02"))
			todayCount, _ := ctl.limiter.OperationCount(ctx, dailyGlobalLimitKey)
			if todayCount > int64(ctl.conf.FreeChatDailyGlobalLimit) {
				log.F(log.M{"ip": client.IP}).Errorf("今日免费次数已用完（全局）")
				return rate.ErrDailyFreeLimitExceeded
			}

			_ = ctl.limiter.OperationIncr(ctx, dailyGlobalLimitKey, time.Hour*24)
		}

		log.F(log.M{"ip": client.IP}).Debugf("free request")
	}

	return nil
}

func (ctl *OpenAIController) saveChatAnswer(ctx context.Context, user *auth.User, replyText string, quotaConsumed int64, realWordCount int, req *chat.Request, questionID int64, chatErrorMessage string) int64 {
	if ctl.conf.EnableRecordChat && !ctl.apiMode {
		answerID, err := ctl.messageRepo.Add(ctx, repo.MessageAddReq{
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

		return answerID
	}
	return 0
}

type QuotaConsume struct {
	InputTokens  int
	OutputTokens int
	InputPrice   float64
	OutputPrice  float64
	PerReqPrice  int64
	TotalPrice   int64
}

func (qc QuotaConsume) TotalTokens() int {
	return qc.InputTokens + qc.OutputTokens
}

func (ctl *OpenAIController) resolveConsumeQuota(req *chat.Request, replyText string, isFreeRequest bool, mod *repo.Model) QuotaConsume {
	inputTokens, _ := chat.MessageTokenCount(req.Messages, req.Model)
	outputTokens, _ := chat.MessageTokenCount(
		chat.Messages{{
			Role:    "assistant",
			Content: replyText,
		}}, req.Model,
	)

	ret := QuotaConsume{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}
	ret.InputPrice, ret.OutputPrice, ret.PerReqPrice, ret.TotalPrice = coins.GetTextModelCoinsDetail(mod.ToCoinModel(), int64(inputTokens), int64(outputTokens))

	// 免费请求，不扣除智慧果
	if isFreeRequest || replyText == "" {
		ret.TotalPrice = 0
	}

	return ret
}

// makeChatQuestionFailed 更新聊天问题为失败状态
func (ctl *OpenAIController) makeChatQuestionFailed(ctx context.Context, questionID int64, err error) {
	if questionID > 0 {
		if err := ctl.messageRepo.UpdateMessageStatus(ctx, questionID, repo.MessageUpdateReq{
			Status: repo.MessageStatusFailed,
			Error:  err.Error(),
		}); err != nil {
			log.WithFields(log.Fields{
				"question_id": questionID,
				"error":       err,
			}).Errorf("update message status failed: %s", err)
		}
	}
}

// saveChatQuestion 保存用户聊天问题
func (ctl *OpenAIController) saveChatQuestion(ctx context.Context, user *auth.User, req *chat.Request) int64 {
	if ctl.conf.EnableRecordChat && !ctl.apiMode {
		lastMessage := req.Messages[len(req.Messages)-1]
		meta := repo.MessageMeta{}

		files := array.Filter(lastMessage.MultipartContents, func(item *chat.MultipartContent, _ int) bool {
			return item.Type == "file" && item.FileURL != nil && item.FileURL.URL != ""
		})
		if len(files) > 0 {
			meta.FileURL = files[0].FileURL.URL
			meta.FileName = files[0].FileURL.Name
		}

		qid, err := ctl.messageRepo.Add(ctx, repo.MessageAddReq{
			UserID:  user.ID,
			Message: lastMessage.Content,
			Role:    repo.MessageRoleUser,
			RoomID:  req.RoomID,
			Model:   req.Model,
			Status:  repo.MessageStatusSucceed,
			Meta:    meta,
		})
		if err != nil {
			log.F(log.M{"req": req, "user_id": user.ID}).Errorf("保存用户聊天请求失败（问题部分）: %s", err)
		}

		return qid
	}

	return 0
}

func (ctl *OpenAIController) loadRoomContextLen(ctx context.Context, roomID int64, userID int64) int64 {
	var maxContextLength int64 = 3
	if roomID > 0 && userID > 0 {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		room, err := ctl.chatSrv.Room(ctx, userID, roomID)
		if err != nil {
			log.F(log.M{"room_id": roomID, "user_id": userID}).Errorf("查询 ROOM 信息失败: %s", err)
		}

		if room != nil && room.MaxContext > 0 {
			maxContextLength = room.MaxContext
		}
	}

	return maxContextLength
}

// 内容安全检测
func (ctl *OpenAIController) contentSafety(req *chat.Request, user *auth.User, sw *streamwriter.StreamWriter) error {
	// API 模式下，不进行内容安全检测
	if ctl.apiMode {
		return nil
	}

	if len(req.Messages) == 0 {
		return nil
	}

	content := req.Messages[len(req.Messages)-1].Content
	if checkRes := ctl.securitySrv.ChatDetect(content); checkRes != nil {
		if checkRes.IsReallyUnSafe() {
			log.F(log.M{"user_id": user.ID, "details": checkRes.ReasonDetail(), "content": content}).Warningf("用户 %d 违规，违规内容：%s", user.ID, checkRes.Reason)
			ctl.sendViolateContentPolicyResp(sw, checkRes.ReasonDetail())
			return errors.New("违规内容")
		}
	}

	return nil
}

const violateContentPolicyMessage = "抱歉，您的请求因包含违规内容被系统拦截，如果您对此有任何疑问或想进一步了解详情，欢迎通过以下渠道与我们联系：\n\n服务邮箱：support@aicode.cc\n\n微博：@mylxsw\n\n客服微信：x-prometheus\n\n\n---\n\n> 本次请求不扣除智慧果。"

func (ctl *OpenAIController) sendViolateContentPolicyResp(sw *streamwriter.StreamWriter, detail string) {
	reason := violateContentPolicyMessage
	if detail != "" {
		reason += fmt.Sprintf("\n> \n> 原因：%s", detail)
	}

	misc.NoError(sw.WriteStream(fmt.Sprintf(
		`{"id":"chatxxx1","object":"chat.completion.chunk","created":%d,"model":"gpt-3.5-turbo-0613","choices":[{"index":0,"delta":{"role":"assistant","content":%s},"finish_reason":null}]}`+"\n\n",
		time.Now().Unix(),
		strconv.Quote(reason),
	)))
}

// Images 图像生成接口，接口参数参考 https://platform.openai.com/docs/api-reference/images/create
func (ctl *OpenAIController) Images(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo) web.Response {
	var req openai.ImageRequest
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	if req.N == 0 {
		req.N = 1
	}

	model := req.Model
	switch model {
	case "dall-e-3":
		if req.Quality == "hd" {
			model = "dall-e-3:hd"
		} else {
			model = "dall-e-3"
		}
	default:
		model = "dall-e-2"
	}

	if ctl.conf.EnableModelRateLimit {
		if err := ctl.limiter.Allow(ctx, fmt.Sprintf("chat-limit:u:%d:m:%s:minute", user.ID, model), redis_rate.PerMinute(5)); err != nil {
			if errors.Is(err, rate.ErrRateLimitExceeded) {
				return webCtx.JSONError("操作频率过高，请稍后再试", http.StatusTooManyRequests)
			}

			log.F(log.M{"user_id": user.ID, "req": req}).Errorf("check rate limit failed: %s", err)
		}
	}

	quota, err := ctl.userSrv.UserQuota(ctx, user.ID)
	if err != nil {
		log.F(log.M{"user_id": user.ID}).Errorf("查询用户智慧果余量失败: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	needCoins := int64(coins.GetUnifiedImageGenCoins(model) * req.N)
	if quota.Rest-quota.Freezed < needCoins {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrQuotaNotEnough), http.StatusPaymentRequired)
	}

	// 冻结本次所需要的智慧果
	if err := ctl.userSrv.FreezeUserQuota(ctx, user.ID, needCoins); err != nil {
		log.F(log.M{"user_id": user.ID, "quota": needCoins}).Errorf("freeze user quota failed: %s", err)
	} else {
		defer func(ctx context.Context) {
			// 解冻智慧果
			if err := ctl.userSrv.UnfreezeUserQuota(ctx, user.ID, needCoins); err != nil {
				log.F(log.M{"user_id": user.ID, "quota": needCoins}).Errorf("unfreeze user quota failed: %s", err)
			}
		}(ctx)
	}

	resp, err := ctl.client.CreateImage(ctx, req)
	if err != nil {
		log.Errorf("createImage error: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	defer func() {
		if err := quotaRepo.QuotaConsume(ctx, user.ID, int64(coins.GetUnifiedImageGenCoins(model)*req.N), repo.NewQuotaUsedMeta("openai-image", model)); err != nil {
			log.Errorf("used quota add failed: %s", err)
		}
	}()

	return webCtx.JSON(resp)
}
