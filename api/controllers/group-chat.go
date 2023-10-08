package controllers

import (
	"context"
	"net/http"
	"strconv"

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
		router.Post("/{group_id}/chat", ctl.Chat)
	})
}

type GroupChatRequest struct {
	Messages  []GroupChatMessage `json:"messages,omitempty"`
	MemberIDs []int64            `json:"member_ids,omitempty"`
}

type GroupChatMember struct {
	ID       int64              `json:"id"`
	Messages []GroupChatMessage `json:"messages"`
}

func (req GroupChatRequest) AvaiableMembers(supportMembers []int64) []GroupChatMember {
	messagesPerMember := req.MessagesPerMembers()
	avaiableIds := array.Filter(array.Intersect(req.MemberIDs, supportMembers), func(id int64, _ int) bool {
		return len(messagesPerMember[id]) > 0
	})

	res := make([]GroupChatMember, 0)
	for memberId, msgs := range messagesPerMember {
		if array.In(memberId, avaiableIds) && len(msgs) > 0 {
			res = append(res, GroupChatMember{
				ID:       memberId,
				Messages: msgs,
			})
		}
	}

	return res
}

func (req GroupChatRequest) MessagesPerMembers() map[int64][]GroupChatMessage {
	messagesPerMembers := make(map[int64][]GroupChatMessage)

	for _, memberID := range req.MemberIDs {
		messages := make([]GroupChatMessage, 0)
		for _, msg := range req.Messages {
			if (msg.Role == "user" || msg.MemberID == memberID) && msg.Content != "" {
				messages = append(messages, msg)
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
		if err == repo.ErrNotFound {
			return webCtx.JSONError("group not found", http.StatusNotFound)
		}

		return webCtx.JSONError("internal server error", http.StatusInternalServerError)
	}

	array.ToMap(grp.Members, func(m model.ChatGroupMember, _ int) int64 {
		return m.Id
	})

	avaiableMembers := req.AvaiableMembers(array.Map(grp.Members, func(m model.ChatGroupMember, _ int) int64 { return m.Id }))
	if len(avaiableMembers) == 0 {
		return webCtx.JSONError("no avaiable members", http.StatusBadRequest)
	}

	// 检查用户当前是否有足够的费用发起本次对话
	membersMap := array.ToMap(grp.Members, func(mem model.ChatGroupMember, _ int) int64 { return mem.Id })
	array.Map(avaiableMembers, func(mem GroupChatMember, _ int) int {
		leftCount, _ := ctl.userSrv.FreeChatRequestCounts(ctx, user.ID, membersMap[mem.ID].ModelId)
		if leftCount > 0 {
			// 免费额度内
			return 0
		}

		// TODO
		return 300

	})

	return webCtx.JSON(web.M{})
}
