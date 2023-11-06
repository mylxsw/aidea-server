package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/internal/misc"

	"github.com/gorilla/websocket"

	"github.com/go-redis/redis_rate/v10"
	"github.com/mylxsw/aidea-server/internal/rate"

	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/aidea-server/internal/ai/streamwriter"

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
	client      openaiHelper.Client      `autowire:"@"`
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
		misc.NoError(uploadedFile.Delete())
		log.Errorf("store file failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	defer func() { misc.NoError(os.Remove(tempPath)) }()

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
func (ctl *OpenAIController) Chat(ctx context.Context, webCtx web.Context, user *auth.User, quotaRepo *repo.QuotaRepo, w http.ResponseWriter, client *auth.ClientInfo) {
	sw, req, err := streamwriter.New[chat.Request](
		webCtx.Input("ws") == "true", ctl.conf.EnableCORS, webCtx.Request().Raw(), w,
	)
	if err != nil {
		log.F(log.M{"user": user.ID, "client": client}).Errorf("create stream writer failed: %s", err)
		return
	}
	defer sw.Close()

	// 请求参数预处理
	maxContextLen := ctl.loadRoomContextLen(ctx, req.RoomID, user.ID)
	var inputTokenCount int64
	req, inputTokenCount, err = req.Fix(ctl.chat, maxContextLen)
	if err != nil {
		misc.NoError(sw.WriteErrorStream(err, http.StatusBadRequest))
		return
	}

	log.F(log.M{"user_id": user.ID, "client": client, "room_id": req.RoomID}).
		Debugf("接收到聊天请求，模型 %s, 上下文消息数量 %d, 输入 token 数量 %d", req.Model, len(req.Messages), inputTokenCount)

	// 基于模型的流控，避免单一模型用户过度使用
	if err := ctl.rateLimitPass(ctx, user, req, sw); err != nil {
		return
	}

	// 免费模型
	// 获取当前用户剩余的智慧果数量，如果不足，则返回错误
	leftCount, maxFreeCount := ctl.userSrv.FreeChatRequestCounts(ctx, user.ID, req.Model)
	if leftCount <= 0 {
		quota, needCoins, err := ctl.queryChatQuota(ctx, quotaRepo, user, sw, webCtx, req, inputTokenCount, maxFreeCount)
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
		if err := ctl.userSrv.FreezeUserQuota(ctx, user.ID, needCoins); err != nil {
			log.F(log.M{"user_id": user.ID, "quota": needCoins}).Errorf("freeze user quota failed: %s", err)
		} else {
			defer func() {
				// 解冻智慧果
				if err := ctl.userSrv.UnfreezeUserQuota(ctx, user.ID, needCoins); err != nil {
					log.F(log.M{"user_id": user.ID, "quota": needCoins}).Errorf("unfreeze user quota failed: %s", err)
				}
			}()
		}
	}

	// 内容安全检测
	if err := ctl.contentSafety(req, user, sw); err != nil {
		return
	}

	// 写入用户消息
	questionID := ctl.saveChatQuestion(ctx, user, req)

	chatCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	var replyText string
	stream, err := ctl.chat.ChatStream(chatCtx, *req)
	if err != nil {
		// 更新问题为失败状态
		ctl.makeChatQuestionFailed(ctx, questionID, err)

		// 内容违反内容安全策略
		if errors.Is(err, chat.ErrContentFilter) {
			ctl.sendViolateContentPolicyResp(sw)
			return
		}

		log.WithFields(log.Fields{"req": req, "user_id": user.ID}).Errorf("聊天请求失败，模型 %s: %v", req.Model, err)

		misc.NoError(sw.WriteErrorStream(errors.New(common.Text(webCtx, ctl.translater, common.ErrInternalError)), http.StatusInternalServerError))
		return
	}

	defer func() {
		var chatErrorMessage string
		if chatError := recover(); chatError != nil {
			chatErrorMessage = fmt.Sprintf("%v", chatError)
		}

		replyText = strings.TrimSpace(replyText)
		// 返回自定义控制信息，告诉客户端当前消耗情况
		realTokenConsumed, quotaConsumed := ctl.resolveConsumeQuota(req, replyText, leftCount > 0)

		// 响应内容为空，报错给客户端
		if replyText == "" && chatErrorMessage == "" {
			chatErrorMessage = "响应内容为空"
		}

		if chatErrorMessage != "" {
			log.F(log.M{"req": req, "user_id": user.ID, "reply": replyText}).Errorf("聊天失败，模型：%s，错误：%s", req.Model, chatErrorMessage)
		}

		func() {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			// 写入用户消息
			answerID := ctl.saveChatAnswer(ctx, user, replyText, quotaConsumed, realTokenConsumed, req, questionID, chatErrorMessage)

			if chatErrorMessage != "" && replyText == "" {
				misc.NoError(sw.WriteErrorStream(errors.New(chatErrorMessage), http.StatusInternalServerError))
			} else {
				// final 消息为定制消息，用于告诉 AIdea 客户端当前的资源消耗情况以及服务端信息
				finalWord := ctl.buildFinalSystemMessage(questionID, answerID, user, quotaConsumed, realTokenConsumed, req, maxContextLen, chatErrorMessage)
				misc.NoError(sw.WriteStream(finalWord))
			}
		}()

		// 更新用户免费聊天次数
		if replyText != "" {
			func() {
				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				if err := ctl.userSrv.UpdateFreeChatCount(ctx, user.ID, req.Model); err != nil {
					log.WithFields(log.Fields{
						"user_id": user.ID,
						"model":   req.Model,
					}).Errorf("update free chat count failed: %s", err)
				}
			}()
		}

		// 扣除智慧果
		if leftCount <= 0 && quotaConsumed > 0 {
			func() {
				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				if err := quotaRepo.QuotaConsume(ctx, user.ID, quotaConsumed, repo.NewQuotaUsedMeta("chat", req.Model)); err != nil {
					log.Errorf("used quota add failed: %s", err)
				}
			}()
		}
	}()

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
			panic("两次响应之间等待时间过长，强制中断")
		case <-ctx.Done():
			return
		case res, ok := <-stream:
			if !ok {
				return
			}

			id++

			if res.ErrorCode != "" {
				log.WithFields(log.Fields{"req": req, "user_id": user.ID}).Errorf("聊天响应失败: %v", res)

				if res.Error != "" {
					res.Text = fmt.Sprintf("\n\n---\n抱歉，我们遇到了一些错误，以下是错误详情：\n%s\n", res.Error)
				} else {
					return
				}
			} else {
				replyText += res.Text
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

			if err := sw.WriteStream(resp); err != nil {
				log.F(log.M{"req": req, "user_id": user.ID}).Warningf("write response failed: %v", err)
				return
			}
		}
	}
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
) openai.ChatCompletionStreamResponse {
	finalMsg := FinalMessage{
		Type:       "summary",
		QuestionID: questionID,
		AnswerID:   answerID,
		Token:      int64(realTokenConsumed),
		Error:      chatErrorMessage,
	}

	if len(req.Messages) >= int(maxContextLen*2) {
		if req.RoomID <= 1 {
			finalMsg.Info = fmt.Sprintf("本次请求消耗了 %d 个 Token。\n\nAI 记住的对话信息越多，消耗的 Token 和智慧果也越多。\n\n如果新问题和之前的对话无关，请在“聊一聊”页面创建新对话。", realTokenConsumed)
		} else {
			finalMsg.Info = fmt.Sprintf("本次请求消耗了 %d 个 Token。\n\nAI 记住的对话信息越多，消耗的 Token 和智慧果也越多。\n\n如果新问题和之前的对话无关，请使用“[新对话](aidea-command://reset-context)”来重置对话上下文。", realTokenConsumed)
		}
	}

	if user.InternalUser() {
		finalMsg.QuotaConsumed = quotaConsumed
	}

	return openai.ChatCompletionStreamResponse{
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
}

// queryChatQuota 检查用户智慧果余量是否足够
func (ctl *OpenAIController) queryChatQuota(
	ctx context.Context,
	quotaRepo *repo.QuotaRepo,
	user *auth.User,
	sw *streamwriter.StreamWriter,
	webCtx web.Context,
	req *chat.Request,
	inputTokenCount int64,
	maxFreeCount int,
) (quota *service.UserQuota, needCoins int64, err error) {
	quota, err = ctl.userSrv.UserQuota(ctx, user.ID)
	if err != nil {
		log.F(log.M{"user_id": user.ID}).Errorf("查询用户智慧果余量失败: %s", err)
		misc.NoError(sw.WriteErrorStream(errors.New(common.Text(webCtx, ctl.translater, common.ErrInternalError)), http.StatusInternalServerError))

		return nil, 0, err
	}

	// 假设本次请求将会消耗 3 个智慧果
	return quota, coins.GetOpenAITextCoins(req.ResolveCalFeeModel(ctl.conf), inputTokenCount) + 3, nil
}

func (ctl *OpenAIController) rateLimitPass(ctx context.Context, user *auth.User, req *chat.Request, sw *streamwriter.StreamWriter) error {
	if ctl.conf.EnableModelRateLimit {
		if err := ctl.limiter.Allow(ctx, fmt.Sprintf("chat-limit:u:%d:m:%s:minute", user.ID, req.Model), redis_rate.PerMinute(5)); err != nil {
			if errors.Is(err, rate.ErrRateLimitExceeded) {
				misc.NoError(sw.WriteErrorStream(errors.New("操作频率过高，请稍后再试"), http.StatusBadRequest))
				return rate.ErrRateLimitExceeded
			}

			log.F(log.M{"user_id": user.ID, "req": req}).Errorf("check rate limit failed: %s", err)
		}
	}

	return nil
}

func (ctl *OpenAIController) saveChatAnswer(ctx context.Context, user *auth.User, replyText string, quotaConsumed int64, realWordCount int, req *chat.Request, questionID int64, chatErrorMessage string) int64 {
	if ctl.conf.EnableRecordChat {
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

func (ctl *OpenAIController) resolveConsumeQuota(req *chat.Request, replyText string, isFreeRequest bool) (int, int64) {
	messages := append(req.Messages, chat.Message{
		Role:    "assistant",
		Content: replyText,
	})

	realTokenConsumed, _ := chat.MessageTokenCount(messages, req.Model)
	quotaConsumed := coins.GetOpenAITextCoins(req.ResolveCalFeeModel(ctl.conf), int64(realTokenConsumed))

	// 免费请求，不扣除智慧果
	if isFreeRequest || replyText == "" {
		quotaConsumed = 0
	}

	return realTokenConsumed, quotaConsumed
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
			log.F(log.M{"req": req, "user_id": user.ID}).Errorf("保存用户聊天请求失败（问题部分）: %s", err)
		}

		return qid
	}

	return 0
}

func (ctl *OpenAIController) loadRoomContextLen(ctx context.Context, roomID int64, userID int64) int64 {
	var maxContextLength int64 = 3
	if roomID > 0 {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		room, err := ctl.chatSrv.Room(ctx, userID, roomID)
		if err != nil {
			log.F(log.M{"room_id": roomID, "user_id": userID}).Errorf("查询 ROOM 信息失败: %s", err)
		}

		if room.MaxContext > 0 {
			maxContextLength = room.MaxContext
		}
	}

	return maxContextLength
}

// 内容安全检测
func (ctl *OpenAIController) contentSafety(req *chat.Request, user *auth.User, sw *streamwriter.StreamWriter) error {
	// content := strings.Join(array.Map(req.Messages, func(msg openai.ChatCompletionMessage, _ int) string { return msg.Content }), "\n")
	content := req.Messages[len(req.Messages)-1].Content
	if checkRes := ctl.securitySrv.ChatDetect(content); checkRes != nil {
		if !checkRes.Safe {
			log.F(log.M{"user_id": user.ID, "details": checkRes, "content": content}).Warningf("用户 %d 违规，违规内容：%s", user.ID, checkRes.Reason)
			ctl.sendViolateContentPolicyResp(sw)
			return errors.New("违规内容")
		}
	}

	return nil
}

const violateContentPolicyMessage = "抱歉，您的请求因包含违规内容被系统拦截，如果您对此有任何疑问或想进一步了解详情，欢迎通过以下渠道与我们联系：\n\n服务邮箱：support@aicode.cc\n\n微博：@mylxsw\n\n客服微信：x-prometheus\n\n\n---\n\n> 本次请求不扣除智慧果。"

func (ctl *OpenAIController) sendViolateContentPolicyResp(sw *streamwriter.StreamWriter) {
	misc.NoError(sw.WriteStream(fmt.Sprintf(
		`{"id":"chatxxx1","object":"chat.completion.chunk","created":%d,"model":"gpt-3.5-turbo-0613","choices":[{"index":0,"delta":{"role":"assistant","content":%s},"finish_reason":null}]}`+"\n\n",
		time.Now().Unix(),
		strconv.Quote(violateContentPolicyMessage),
	)))
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
