package controllers

import (
	"context"
	"errors"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/internal/ai/chat"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/asteria/log"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	repo        *repo.Repository         `autowire:"@"`
	queue       *queue.Queue             `autowire:"@"`
	securitySrv *service.SecurityService `autowire:"@"`
	userSrv     *service.UserService     `autowire:"@"`
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
		router.Delete("/{group_id}", ctl.DeleteGroup)
		router.Get("/{group_id}/messages", ctl.GroupMessages)
		router.Post("/{group_id}/chat", ctl.Chat)
		router.Delete("/{group_id}/chat/{message_id}", ctl.DeleteMessage)

		router.Get("/{group_id}/chat-messages", ctl.ChatMessageStatus)
	})
}

type GroupCreateRequest struct {
	Name    string        `json:"name"`
	Members []repo.Member `json:"members"`
}

// CreateGroup 创建群组
func (ctl *GroupChatController) CreateGroup(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	var req GroupCreateRequest
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}
	req.Name = strings.TrimSpace(req.Name)

	if len(req.Members) == 0 {
		return webCtx.JSONError("empty members", http.StatusBadRequest)
	}

	if req.Name == "" {
		return webCtx.JSONError("empty group name", http.StatusBadRequest)
	}

	groupID, err := ctl.repo.ChatGroup.CreateGroup(ctx, user.ID, req.Name, req.Members)
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

// Group 获取群组信息
func (ctl *GroupChatController) Group(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
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

	return webCtx.JSON(grp)
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

	page := webCtx.Int64Input("page", 1)
	if page < 1 || page > 1000 {
		page = 1
	}

	perPage := webCtx.Int64Input("per_page", 100)
	if perPage < 1 || perPage > 300 {
		perPage = 100
	}

	messages, meta, err := ctl.repo.ChatGroup.GetChatMessages(ctx, int64(groupID), user.ID, page, perPage)
	if err != nil {
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data":      messages,
		"page":      meta.Page,
		"per_page":  meta.PerPage,
		"total":     meta.Total,
		"last_page": meta.LastPage,
	})
}

type GroupChatRequest struct {
	Messages  []GroupChatMessage `json:"messages,omitempty"`
	MemberIDs []int64            `json:"member_ids,omitempty"`
}

type GroupChatMember struct {
	ID       int64         `json:"id"`
	Messages chat.Messages `json:"messages"`
}

func (req GroupChatRequest) AvailableMembers(supportMembers []int64) []GroupChatMember {
	messagesPerMember := req.MessagesPerMembers()
	availableIds := array.Filter(array.Intersect(req.MemberIDs, supportMembers), func(id int64, _ int) bool {
		return len(messagesPerMember[id]) > 0
	})

	res := make([]GroupChatMember, 0)
	for memberId, msgs := range messagesPerMember {
		if array.In(memberId, availableIds) && len(msgs) > 0 {
			res = append(res, GroupChatMember{
				ID:       memberId,
				Messages: msgs,
			})
		}
	}

	return res
}

func (req GroupChatRequest) MessagesPerMembers() map[int64]chat.Messages {
	messagesPerMembers := make(map[int64]chat.Messages)

	for _, memberID := range req.MemberIDs {
		messages := make(chat.Messages, 0)
		for _, msg := range req.Messages {
			if (msg.Role == "user" || msg.MemberID == memberID) && msg.Content != "" {
				messages = append(messages, chat.Message{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}

		messagesPerMembers[memberID] = messages
	}

	return messagesPerMembers
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

	if len(req.Messages) == 0 {
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

	availableMembers := req.AvailableMembers(array.Map(grp.Members, func(m model.ChatGroupMember, _ int) int64 { return m.Id }))
	if len(availableMembers) == 0 {
		return webCtx.JSONError("no available members", http.StatusBadRequest)
	}

	messagesPerMembers := req.MessagesPerMembers()

	// 检查用户当前是否有足够的费用发起本次对话
	membersMap := array.ToMap(grp.Members, func(mem model.ChatGroupMember, _ int) int64 { return mem.Id })
	coinCounts := array.Map(availableMembers, func(mem GroupChatMember, _ int) int64 {
		leftCount, _ := ctl.userSrv.FreeChatRequestCounts(ctx, user.ID, membersMap[mem.ID].ModelId)
		if leftCount > 0 {
			// 免费额度内
			return 0
		}

		count, err := chat.MessageTokenCount(messagesPerMembers[mem.ID], membersMap[mem.ID].ModelId)
		if err != nil {
			log.With(mem).Errorf("calc message token count failed: %v", err)
			return coins.GetOpenAITextCoins(membersMap[mem.ID].ModelId, 1000)
		}

		return coins.GetOpenAITextCoins(membersMap[mem.ID].ModelId, int64(count))
	})

	needCoins := array.Reduce(coinCounts, func(carry, item int64) int64 { return carry + item }, int64(len(coinCounts)*10))
	quota, err := ctl.repo.Quota.GetUserQuota(ctx, user.ID)
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	// 获取当前用户剩余的智慧果数量，如果不足，则返回错误
	// 假设当前响应消耗 2 个智慧果
	restQuota := quota.Quota - quota.Used - needCoins
	if restQuota <= 0 {
		return webCtx.JSONError(common.ErrQuotaNotEnough, http.StatusPaymentRequired)
	}

	questionID, err := ctl.repo.ChatGroup.AddChatMessage(ctx, grp.Group.Id, user.ID, repo.ChatGroupMessage{
		Message: req.Messages[len(req.Messages)-1].Content,
		Role:    int64(repo.MessageRoleUser),
		Status:  repo.ChatGroupMessageStatusSucceed,
	})
	if err != nil {
		log.With(req).Errorf("add chat message failed: %s", err)
		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	tasks := make([]GroupChatTask, 0)
	for memberID, msg := range messagesPerMembers {
		answerID, err := ctl.repo.ChatGroup.AddChatMessage(ctx, grp.Group.Id, user.ID, repo.ChatGroupMessage{
			Role:     int64(repo.MessageRoleAssistant),
			Pid:      questionID,
			MemberId: memberID,
			Status:   repo.ChatGroupMessageStatusWaiting,
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
