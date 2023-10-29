package controllers

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/asteria/log"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/repo/model"
	"github.com/mylxsw/aidea-server/internal/service"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

type GroupChatController struct {
	conf    *config.Config       `autowire:"@"`
	repo    *repo.Repository     `autowire:"@"`
	queue   *queue.Queue         `autowire:"@"`
	userSrv *service.UserService `autowire:"@"`
}

func NewGroupChatController(resolver infra.Resolver) web.Controller {
	ctl := &GroupChatController{}
	resolver.MustAutoWire(ctl)
	return ctl
}

func (ctl *GroupChatController) Register(router web.Router) {
	router.Group("/group-chat", func(router web.Router) {
		router.Get("/", ctl.Groups)
		router.Post("/", ctl.CreateGroup)
		router.Get("/{group_id}", ctl.Group)
		router.Put("/{group_id}", ctl.UpdateGroup)
		router.Delete("/{group_id}", ctl.DeleteGroup)
		router.Get("/{group_id}/messages", ctl.GroupMessages)
		router.Post("/{group_id}/chat", ctl.Chat)
		router.Post("/{group_id}/chat-system", ctl.ChatSystem)
		router.Delete("/{group_id}/chat/{message_id}", ctl.DeleteMessage)
		router.Delete("/{group_id}/all-chat", ctl.DeleteAllMessages)

		router.Get("/{group_id}/chat-messages", ctl.ChatMessageStatus)
	})
}

type GroupCreateRequest struct {
	Name      string        `json:"name"`
	AvatarURL string        `json:"avatar_url,omitempty"`
	Members   []repo.Member `json:"members,omitempty"`
}

// CreateGroup 创建群组
func (ctl *GroupChatController) CreateGroup(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	var req GroupCreateRequest
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}
	req.Name = strings.TrimSpace(req.Name)

	if len(req.Members) == 0 {
		req.Members = array.Map(
			array.Filter(chat.Models(ctl.conf, false), func(m chat.Model, _ int) bool {
				return true
			}),
			func(m chat.Model, _ int) repo.Member {
				return repo.Member{
					ModelID:   m.RealID(),
					ModelName: m.ShortName,
				}
			},
		)
	}

	req.Members = array.Map(req.Members, func(mem repo.Member, _ int) repo.Member {
		segs := strings.Split(mem.ModelID, ":")
		if len(segs) == 2 {
			mem.ModelID = segs[1]
		}

		return mem
	})

	if req.Name == "" {
		return webCtx.JSONError("empty group name", http.StatusBadRequest)
	}

	groupID, err := ctl.repo.ChatGroup.CreateGroup(ctx, user.ID, req.Name, req.AvatarURL, req.Members)
	if err != nil {
		log.F(log.M{
			"name":    req.Name,
			"members": req.Members,
			"user_id": user.ID,
		}).Errorf("create group failed: %s", err)
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"group_id": groupID,
	})
}

// Groups 获取用户的群组列表
func (ctl *GroupChatController) Groups(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	groups, err := ctl.repo.ChatGroup.Groups(ctx, user.ID, RoomsQueryLimit)
	if err != nil {
		log.Errorf("get groups failed: %s", err)
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data": groups,
	})
}

type GroupMember struct {
	ID        int64  `json:"id"`
	ModelId   string `json:"model_id,omitempty"`
	ModelName string `json:"model_name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Status    int64  `json:"status,omitempty"`
}

// Group 获取群组信息
func (ctl *GroupChatController) Group(ctx context.Context, webCtx web.Context, user *auth.User, client *auth.ClientInfo) web.Response {
	groupID, err := strconv.Atoi(webCtx.PathVar("group_id"))
	if err != nil {
		return webCtx.JSONError("invalid group id", http.StatusBadRequest)
	}

	grp, err := ctl.repo.ChatGroup.GetGroup(ctx, int64(groupID), user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return webCtx.JSONError("group not found", http.StatusNotFound)
		}

		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	models := array.ToMap(chat.Models(ctl.conf, true), func(item chat.Model, _ int) string {
		return item.RealID()
	})

	return webCtx.JSON(web.M{
		"group": grp.Group,
		"members": array.Map(
			grp.Members,
			func(mem model.ChatGroupMember, _ int) GroupMember {
				return GroupMember{
					ID:        mem.Id,
					ModelId:   mem.ModelId,
					ModelName: mem.ModelName,
					AvatarURL: models[mem.ModelId].AvatarURL,
					Status:    mem.Status,
				}
			},
		),
	})
}

// UpdateGroup 更新群组
func (ctl *GroupChatController) UpdateGroup(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	groupID, err := strconv.Atoi(webCtx.PathVar("group_id"))
	if err != nil {
		return webCtx.JSONError("invalid group id", http.StatusBadRequest)
	}

	var req GroupCreateRequest
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	if req.Name == "" {
		return webCtx.JSONError("empty group name", http.StatusBadRequest)
	}

	if err := ctl.repo.ChatGroup.UpdateGroup(ctx, int64(groupID), user.ID, req.Name, req.AvatarURL); err != nil {
		log.With(req).Errorf("update group %d failed: %s", groupID, err)
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	if len(req.Members) > 0 {
		if err := ctl.repo.ChatGroup.UpdateGroupMembers(ctx, int64(groupID), user.ID, req.Members); err != nil {
			log.With(req).Errorf("update group %d members failed: %s", groupID, err)
			return webCtx.JSONError("internal server error", http.StatusInternalServerError)
		}
	}

	return webCtx.JSON(web.M{})
}

// DeleteGroup 删除群组
func (ctl *GroupChatController) DeleteGroup(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	groupID, err := strconv.Atoi(webCtx.PathVar("group_id"))
	if err != nil {
		return webCtx.JSONError("invalid group id", http.StatusBadRequest)
	}

	if err := ctl.repo.ChatGroup.DeleteGroup(ctx, int64(groupID), user.ID, false); err != nil {
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// GroupMessages 获取群组消息
func (ctl *GroupChatController) GroupMessages(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	groupID, err := strconv.Atoi(webCtx.PathVar("group_id"))
	if err != nil {
		return webCtx.JSONError("invalid group id", http.StatusBadRequest)
	}

	startID := webCtx.Int64Input("start_id", 0)
	perPage := webCtx.Int64Input("per_page", 100)
	if perPage < 1 || perPage > 300 {
		perPage = 100
	}

	messages, lastID, err := ctl.repo.ChatGroup.GetChatMessages(ctx, int64(groupID), user.ID, startID, perPage)
	if err != nil {
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data":     messages,
		"start_id": startID,
		"last_id":  lastID,
		"per_page": perPage,
	})
}

type GroupChatRequest struct {
	Message   string  `json:"message,omitempty"`
	MemberIDs []int64 `json:"member_ids,omitempty"`
}

type GroupChatMember struct {
	ID       int64         `json:"id"`
	Messages chat.Messages `json:"messages"`
}

func (req GroupChatRequest) AvailableMembers(supportMembers []int64) []int64 {
	return array.Intersect(req.MemberIDs, supportMembers)
}

type GroupChatMessage struct {
	Role     string `json:"role,omitempty"`
	Content  string `json:"content,omitempty"`
	MemberID int64  `json:"member_id,omitempty"`
}

type GroupChatTask struct {
	MemberID int64  `json:"member_id"`
	TaskID   string `json:"task_id"`
	AnswerID int64  `json:"answer_id"`
}

type Question struct {
	Question string                     `json:"question"`
	Answers  []repo.ChatGroupMessageRes `json:"answers"`
}

// Chat 发起聊天
func (ctl *GroupChatController) Chat(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	groupID, err := strconv.Atoi(webCtx.PathVar("group_id"))
	if err != nil {
		return webCtx.JSONError("invalid group id", http.StatusBadRequest)
	}

	var req GroupChatRequest
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	if req.Message == "" {
		return webCtx.JSONError("empty messages", http.StatusBadRequest)
	}

	// 查询群组信息
	grp, err := ctl.repo.ChatGroup.GetGroup(ctx, int64(groupID), user.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return webCtx.JSONError("group not found", http.StatusNotFound)
		}

		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	failedMessageWriter := func() int64 {
		questionID, err := ctl.repo.ChatGroup.AddChatMessage(ctx, grp.Group.Id, user.ID, repo.ChatGroupMessage{
			Message: req.Message,
			Role:    int64(repo.MessageRoleUser),
			Status:  repo.MessageStatusFailed,
		})
		if err != nil {
			log.With(req).Errorf("add chat message failed: %s", err)
			return 0
		}

		return questionID
	}

	// 如果没有指定对话的成员，则随机选择一个
	if len(req.MemberIDs) == 0 {
		req.MemberIDs = []int64{grp.Members[rand.Intn(len(grp.Members))].Id}
	}

	availableMembers := req.AvailableMembers(array.Map(grp.Members, func(m model.ChatGroupMember, _ int) int64 { return m.Id }))
	if len(availableMembers) == 0 {
		failedMessageWriter()
		return webCtx.JSONError("no available members", http.StatusBadRequest)
	}

	// 每个成员的聊天上下文
	contextMessages, _, err := ctl.repo.ChatGroup.GetChatMessages(ctx, grp.Group.Id, user.ID, 0, 100)
	if err != nil {
		questionID := failedMessageWriter()
		log.F(log.M{
			"req":         req,
			"question_id": questionID,
		}).Errorf("query chat context failed: %s", err)
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	qas := buildQuestionFromChatGroupMessages(contextMessages)
	messagesPerMembers := make(map[int64]chat.Messages)
	for _, memberID := range availableMembers {
		memberMessages := make(chat.Messages, 0)
		for _, qa := range qas {
			memberMessages = append(memberMessages, chat.Message{Role: "user", Content: qa.Question})
			// 从多个回复中选择一个，选择策略如下
			// 1. 如果有当前 member_id 的回复，优先选择
			// 2. 没有当前 member_id 的回复，则随便选择一个
			selectedAnswer := array.Filter(qa.Answers, func(ans repo.ChatGroupMessageRes, _ int) bool { return ans.MemberId == memberID })
			if len(selectedAnswer) == 0 {
				selectedAnswer = qa.Answers
			}

			if len(selectedAnswer) == 0 {
				continue
			}

			memberMessages = append(memberMessages, chat.Message{Role: "assistant", Content: selectedAnswer[0].Message})
		}

		memberMessages = append(memberMessages, chat.Message{Role: "user", Content: req.Message})
		messagesPerMembers[memberID] = memberMessages
	}

	log.With(messagesPerMembers).Debugf("group chat messages per members")

	// 检查用户当前是否有足够的费用发起本次对话
	membersMap := array.ToMap(grp.Members, func(mem model.ChatGroupMember, _ int) int64 { return mem.Id })
	coinCounts := array.Map(availableMembers, func(memID int64, _ int) int64 {
		leftCount, _ := ctl.userSrv.FreeChatRequestCounts(ctx, user.ID, membersMap[memID].ModelId)
		if leftCount > 0 {
			// 免费额度内
			return 0
		}

		count, err := chat.MessageTokenCount(messagesPerMembers[memID], membersMap[memID].ModelId)
		if err != nil {
			log.F(log.M{"member_id": memID, "req": req}).Errorf("calc message token count failed: %v", err)
			return coins.GetOpenAITextCoins(membersMap[memID].ModelId, 1000)
		}

		return coins.GetOpenAITextCoins(membersMap[memID].ModelId, int64(count)) + 2
	})

	needCoins := array.Reduce(coinCounts, func(carry, item int64) int64 { return carry + item }, 0)
	quota, err := ctl.repo.Quota.GetUserQuota(ctx, user.ID)
	if err != nil {
		questionID := failedMessageWriter()
		log.F(log.M{
			"req":         req,
			"question_id": questionID,
		}).Errorf("get user quota failed: %s", err)
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	// 获取当前用户剩余的智慧果数量，如果不足，则返回错误
	restQuota := quota.Quota - quota.Used - needCoins

	log.F(log.M{
		"need_coins": needCoins,
		"rest_quota": quota.Quota - quota.Used,
	}).Debugf("group chat consume estimate")

	if restQuota < 0 {
		failedMessageWriter()
		return webCtx.JSONError(common.ErrQuotaNotEnough, http.StatusPaymentRequired)
	}

	// 记录用户提问问题
	questionID, err := ctl.repo.ChatGroup.AddChatMessage(ctx, grp.Group.Id, user.ID, repo.ChatGroupMessage{
		Message: req.Message,
		Role:    int64(repo.MessageRoleUser),
		Status:  repo.MessageStatusSucceed,
	})
	if err != nil {
		log.With(req).Errorf("add chat message failed: %s", err)
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	// 为每一个成员创建聊天记录（待处理任务）
	tasks := make([]GroupChatTask, 0)
	for memberID, msg := range messagesPerMembers {
		answerID, err := ctl.repo.ChatGroup.AddChatMessage(ctx, grp.Group.Id, user.ID, repo.ChatGroupMessage{
			Role:     int64(repo.MessageRoleAssistant),
			Pid:      questionID,
			MemberId: memberID,
			Status:   repo.MessageStatusWaiting,
		})
		if err != nil {
			log.With(req).Errorf("add chat message failed: %s", err)
			continue
		}

		// 将消息放入队列中，等待处理
		payload := queue.GroupChatPayload{
			UserID:          user.ID,
			GroupID:         grp.Group.Id,
			MemberID:        memberID,
			QuestionID:      questionID,
			MessageID:       answerID,
			ModelID:         membersMap[memberID].ModelId,
			ContextMessages: msg,
			CreatedAt:       time.Now(),
		}

		// 加入异步任务队列
		taskID, err := ctl.queue.Enqueue(&payload, queue.NewGroupChatTask)
		if err != nil {
			log.With(payload).Errorf("enqueue chat task failed: %s", err)
			continue
		}

		tasks = append(tasks, GroupChatTask{
			MemberID: memberID,
			TaskID:   taskID,
			AnswerID: answerID,
		})
	}

	return webCtx.JSON(web.M{
		"tasks":       tasks,
		"question_id": questionID,
	})
}

// ChatSystem 发起系统消息
func (ctl *GroupChatController) ChatSystem(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	groupID, err := strconv.Atoi(webCtx.PathVar("group_id"))
	if err != nil {
		return webCtx.JSONError("invalid group id", http.StatusBadRequest)
	}

	messageType := repo.ResolveGroupMessageTypeToRole(webCtx.Input("message_type"))
	message := webCtx.Input("message")

	if messageType == 0 {
		return webCtx.JSONError("invalid message type", http.StatusBadRequest)
	}

	questionID, err := ctl.repo.ChatGroup.AddChatMessage(ctx, int64(groupID), user.ID, repo.ChatGroupMessage{
		Message: message,
		Role:    messageType,
		Status:  repo.MessageStatusSucceed,
	})
	if err != nil {
		log.F(log.M{
			"group_id":     groupID,
			"message_type": messageType,
			"message":      message,
		}).Errorf("add chat system message failed: %s", err)
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"data": repo.ChatGroupMessageRes{
		ChatGroupMessage: model.ChatGroupMessage{
			Id:        questionID,
			Message:   message,
			Role:      1,
			Status:    repo.MessageStatusSucceed,
			UserId:    user.ID,
			GroupId:   int64(groupID),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Type: repo.ResolveGroupMessageType(messageType),
	}})
}

// DeleteMessage 删除消息
func (ctl *GroupChatController) DeleteMessage(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	groupID, err := strconv.Atoi(webCtx.PathVar("group_id"))
	if err != nil {
		return webCtx.JSONError("invalid group id", http.StatusBadRequest)
	}

	messageID, err := strconv.Atoi(webCtx.PathVar("message_id"))
	if err != nil {
		return webCtx.JSONError("invalid message id", http.StatusBadRequest)
	}

	if err := ctl.repo.ChatGroup.DeleteChatMessage(ctx, int64(groupID), user.ID, int64(messageID)); err != nil {
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// DeleteAllMessages 删除所有消息
func (ctl *GroupChatController) DeleteAllMessages(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	groupID, err := strconv.Atoi(webCtx.PathVar("group_id"))
	if err != nil {
		return webCtx.JSONError("invalid group id", http.StatusBadRequest)
	}

	if err := ctl.repo.ChatGroup.DeleteAllChatMessage(ctx, int64(groupID), user.ID); err != nil {
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// ChatMessageStatus 查询聊天任务状态
func (ctl *GroupChatController) ChatMessageStatus(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	groupID, err := strconv.Atoi(webCtx.PathVar("group_id"))
	if err != nil {
		return webCtx.JSONError("invalid group id", http.StatusBadRequest)
	}

	messageIDs := array.Map(strings.Split(webCtx.Input("message_ids"), ","), func(item string, _ int) int64 {
		val, _ := strconv.Atoi(item)
		return int64(val)
	})

	messages, err := ctl.repo.ChatGroup.GetChatMessagesStatus(ctx, int64(groupID), user.ID, messageIDs)
	if err != nil {
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data": messages,
	})
}

func buildQuestionFromChatGroupMessages(contextMessages []repo.ChatGroupMessageRes) []Question {
	cutoffIndex := -1
	for i := 0; i < len(contextMessages); i++ {
		if repo.ResolveGroupMessageType(contextMessages[i].Role) == "contextBreak" {
			cutoffIndex = i
			break
		}
	}

	if cutoffIndex >= 0 {
		contextMessages = contextMessages[:cutoffIndex]
	}

	questions := array.Reverse(array.Filter(contextMessages, func(msg repo.ChatGroupMessageRes, _ int) bool { return msg.Role == int64(repo.MessageRoleUser) }))
	answers := array.GroupBy(
		array.Reverse(array.Filter(contextMessages, func(msg repo.ChatGroupMessageRes, _ int) bool {
			return msg.Role == int64(repo.MessageRoleAssistant) && msg.Status == int64(repo.MessageStatusSucceed)
		})),
		func(msg repo.ChatGroupMessageRes) int64 { return msg.Pid },
	)

	qas := make([]Question, 0)
	for _, q := range questions {
		qa := Question{Question: q.Message}
		if ans, ok := answers[q.Id]; ok {
			qa.Answers = ans
		}

		if len(qa.Answers) > 0 {
			qas = append(qas, qa)
		}
	}

	// 只保留最后的 5 条聊天记录
	if len(qas) > 5 {
		qas = qas[len(qas)-5:]
	}

	return qas
}
