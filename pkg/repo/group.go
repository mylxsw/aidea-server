package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	model2 "github.com/mylxsw/aidea-server/pkg/repo/model"
	"time"

	"github.com/mylxsw/eloquent"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/go-utils/array"
	"gopkg.in/guregu/null.v3"
)

type ChatGroupRepo struct {
	db *sql.DB
}

func NewChatGroupRepo(db *sql.DB) *ChatGroupRepo {
	return &ChatGroupRepo{db: db}
}

type Member struct {
	ID        int    `json:"id,omitempty"`
	ModelID   string `json:"model_id"`
	ModelName string `json:"model_name,omitempty"`
}

const (
	// ChatGroupMemberStatusNormal 组成员状态：正常
	ChatGroupMemberStatusNormal = 1
	// ChatGroupMemberStatusDeleted 组成员状态：已删除
	ChatGroupMemberStatusDeleted = 2

	// MessageStatusWaiting 消息状态：待处理
	MessageStatusWaiting = 0
	// MessageStatusSucceed 消息状态：成功
	MessageStatusSucceed = 1
	// MessageStatusFailed 消息状态：失败
	MessageStatusFailed = 2
)

// CreateGroup 创建一个聊天群组
func (repo *ChatGroupRepo) CreateGroup(ctx context.Context, userID int64, name string, avatarURL string, members []Member) (int64, error) {
	var groupID int64
	err := eloquent.Transaction(repo.db, func(tx query.Database) error {
		gid, err := model2.NewRoomsModel(tx).Create(ctx, query.KV{
			model2.FieldRoomsUserId:         userID,
			model2.FieldRoomsName:           name,
			model2.FieldRoomsAvatarUrl:      avatarURL,
			model2.FieldRoomsPriority:       null.IntFrom(0),
			model2.FieldRoomsRoomType:       null.IntFrom(RoomTypeGroupChat),
			model2.FieldRoomsCreatedAt:      null.TimeFrom(time.Now()),
			model2.FieldRoomsUpdatedAt:      null.TimeFrom(time.Now()),
			model2.FieldRoomsLastActiveTime: null.TimeFrom(time.Now()),
		})
		if err != nil {
			return fmt.Errorf("create group failed: %w", err)
		}

		groupID = gid

		for _, member := range members {
			if _, err := model2.NewChatGroupMemberModel(tx).Create(ctx, query.KV{
				model2.FieldChatGroupMemberGroupId:   gid,
				model2.FieldChatGroupMemberUserId:    userID,
				model2.FieldChatGroupMemberModelId:   member.ModelID,
				model2.FieldChatGroupMemberModelName: member.ModelName,
				model2.FieldChatGroupMemberStatus:    ChatGroupMemberStatusNormal,
			}); err != nil {
				return fmt.Errorf("create group member failed: %w", err)
			}
		}

		return nil
	})

	return groupID, err
}

// UpdateGroup 更新群组信息
func (repo *ChatGroupRepo) UpdateGroup(ctx context.Context, groupID int64, userID int64, name, avatarURL string) error {
	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().Where(model2.FieldRoomsId, groupID).Where(model2.FieldRoomsUserId, userID)
		grp, err := model2.NewRoomsModel(tx).First(ctx, q)
		if err != nil {
			if errors.Is(err, query.ErrNoResult) {
				return ErrNotFound
			}

			return fmt.Errorf("query group failed: %w", err)
		}

		grp.Name = null.StringFrom(name)
		grp.AvatarUrl = null.StringFrom(avatarURL)

		if err := grp.Save(ctx); err != nil {
			return fmt.Errorf("save group failed: %w", err)
		}

		return nil
	})
}

// UpdateGroupMembers 更新群组成员
func (repo *ChatGroupRepo) UpdateGroupMembers(ctx context.Context, groupID int64, userID int64, members []Member) error {
	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().Where(model2.FieldChatGroupMemberGroupId, groupID).
			Where(model2.FieldChatGroupMemberUserId, userID)
		currentMembers, err := model2.NewChatGroupMemberModel(tx).Get(ctx, q)
		if err != nil {
			return fmt.Errorf("query group members failed: %w", err)
		}

		membersMap := array.ToMap(members, func(member Member, _ int) string { return member.ModelID })
		currentMembersMap := array.ToMap(currentMembers, func(member model2.ChatGroupMemberN, _ int) string { return member.ModelId.ValueOrZero() })

		for i, member := range currentMembers {
			if modifyMember, ok := membersMap[member.ModelId.ValueOrZero()]; !ok {
				// 1. 删除已经不存在的成员
				currentMembers[i].Status = null.IntFrom(ChatGroupMemberStatusDeleted)
			} else {
				// 2. 更新已经存在的成员
				member.ModelId = null.StringFrom(modifyMember.ModelID)
				member.ModelName = null.StringFrom(modifyMember.ModelName)
				member.Status = null.IntFrom(ChatGroupMemberStatusNormal)
				currentMembers[i] = member
			}
		}

		// 3. 添加新成员
		for _, member := range members {
			if _, ok := currentMembersMap[member.ModelID]; !ok {
				mem := model2.ChatGroupMemberN{
					GroupId:   null.IntFrom(groupID),
					UserId:    null.IntFrom(userID),
					ModelId:   null.StringFrom(member.ModelID),
					ModelName: null.StringFrom(member.ModelName),
					Status:    null.IntFrom(ChatGroupMemberStatusNormal),
				}

				mem.SetModel(model2.NewChatGroupMemberModel(tx))
				currentMembers = append(currentMembers, mem)
			}
		}

		// 4. 保存
		for _, member := range currentMembers {
			if err := member.Save(ctx); err != nil {
				return fmt.Errorf("save group member failed: %w", err)
			}
		}

		return nil
	})
}

// AddMembersToGroup 添加成员到群组
func (repo *ChatGroupRepo) AddMembersToGroup(ctx context.Context, groupID, userID int64, members []Member) error {
	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		for _, member := range members {
			if _, err := model2.NewChatGroupMemberModel(tx).Create(ctx, query.KV{
				model2.FieldChatGroupMemberGroupId:   groupID,
				model2.FieldChatGroupMemberModelId:   member.ModelID,
				model2.FieldChatGroupMemberModelName: member.ModelName,
				model2.FieldChatGroupMemberStatus:    ChatGroupMemberStatusNormal,
			}); err != nil {
				return fmt.Errorf("create group member failed: %w", err)
			}
		}

		return nil
	})
}

// RemoveMembersFromGroup 从群组中移除成员
func (repo *ChatGroupRepo) RemoveMembersFromGroup(ctx context.Context, groupID, userID int64, memberIDs []int64) error {
	if len(memberIDs) == 0 {
		return nil
	}

	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().Where(model2.FieldChatGroupMemberGroupId, groupID).
			Where(model2.FieldChatGroupMemberUserId, userID).
			Where(model2.FieldChatGroupMemberStatus, ChatGroupMemberStatusNormal).
			WhereIn(model2.FieldChatGroupMemberId, memberIDs)

		_, err := model2.NewChatGroupMemberModel(tx).UpdateFields(ctx, query.KV{model2.FieldChatGroupMemberStatus: ChatGroupMemberStatusDeleted}, q)
		return err
	})
}

type Group struct {
	Group   model2.Rooms             `json:"group"`
	Members []model2.ChatGroupMember `json:"members"`
}

// GetGroup 获取群组信息
func (repo *ChatGroupRepo) GetGroup(ctx context.Context, groupID int64, userID int64) (*Group, error) {
	// 1. 获取群组信息
	grp, err := model2.NewRoomsModel(repo.db).First(ctx, query.Builder().
		Where(model2.FieldRoomsId, groupID).
		Where(model2.FieldRoomsUserId, userID))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query group failed: %w", err)
	}

	// 2. 获取群组成员信息
	members, err := model2.NewChatGroupMemberModel(repo.db).Get(ctx, query.Builder().
		Where(model2.FieldChatGroupMemberGroupId, groupID).
		Where(model2.FieldChatGroupMemberUserId, userID))
	if err != nil {
		return nil, fmt.Errorf("query group members failed: %w", err)
	}

	return &Group{
		Group: grp.ToRooms(),
		Members: array.Map(members, func(member model2.ChatGroupMemberN, _ int) model2.ChatGroupMember {
			return member.ToChatGroupMember()
		}),
	}, nil
}

// Groups 获取用户的群组列表
func (repo *ChatGroupRepo) Groups(ctx context.Context, userID int64, limit int64) ([]model2.Rooms, error) {
	groups, err := model2.NewRoomsModel(repo.db).Get(ctx, query.Builder().
		Where(model2.FieldRoomsUserId, userID).
		WhereIn(model2.FieldRoomsRoomType, []int64{RoomTypeGroupChat}).
		OrderBy(model2.FieldRoomsUpdatedAt, "DESC").
		Limit(limit))
	if err != nil {
		return nil, fmt.Errorf("query groups failed: %w", err)
	}

	return array.Map(groups, func(group model2.RoomsN, _ int) model2.Rooms {
		return group.ToRooms()
	}), nil
}

type ChatGroupMessage struct {
	Message       string `json:"message,omitempty"`
	Role          int64  `json:"role,omitempty"`
	TokenConsumed int64  `json:"token_consumed,omitempty"`
	QuotaConsumed int64  `json:"quota_consumed,omitempty"`
	Pid           int64  `json:"pid,omitempty"`
	MemberId      int64  `json:"member_id,omitempty"`
	Status        int64  `json:"status,omitempty"`
	Error         string `json:"error,omitempty"`
}

// AddChatMessage 添加聊天消息
func (repo *ChatGroupRepo) AddChatMessage(ctx context.Context, groupID, userID int64, msg ChatGroupMessage) (int64, error) {
	var messageID int64
	err := eloquent.Transaction(repo.db, func(tx query.Database) error {
		if MessageRole(msg.Role) == MessageRoleUser {
			if _, err := model2.NewRoomsModel(tx).UpdateFields(
				ctx,
				query.KV{
					model2.FieldRoomsLastActiveTime: null.TimeFrom(time.Now()),
					model2.FieldRoomsDescription:    null.StringFrom(misc.SubString(msg.Message, 70)),
				},
				query.Builder().Where(model2.FieldRoomsId, groupID),
			); err != nil {
				return fmt.Errorf("update group last active time failed: %w", err)
			}
		}

		chatMsg := model2.ChatGroupMessage{
			GroupId:       groupID,
			UserId:        userID,
			Message:       msg.Message,
			Role:          msg.Role,
			TokenConsumed: msg.TokenConsumed,
			QuotaConsumed: msg.QuotaConsumed,
			Pid:           msg.Pid,
			MemberId:      msg.MemberId,
			Status:        msg.Status,
			Error:         msg.Error,
		}

		msgID, err := model2.NewChatGroupMessageModel(tx).Save(ctx, chatMsg.ToChatGroupMessageN(
			model2.FieldChatGroupMessageGroupId,
			model2.FieldChatGroupMessageUserId,
			model2.FieldChatGroupMessageMessage,
			model2.FieldChatGroupMessageRole,
			model2.FieldChatGroupMessageTokenConsumed,
			model2.FieldChatGroupMessageQuotaConsumed,
			model2.FieldChatGroupMessagePid,
			model2.FieldChatGroupMessageMemberId,
			model2.FieldChatGroupMessageStatus,
			model2.FieldChatGroupMessageError,
		))
		if err != nil {
			return fmt.Errorf("save chat message failed: %w", err)
		}

		messageID = msgID

		return nil
	})

	return messageID, err
}

// GetChatMessage 获取聊天消息
func (repo *ChatGroupRepo) GetChatMessage(ctx context.Context, groupID, userID, messageID int64) (*model2.ChatGroupMessage, error) {
	q := query.Builder().
		Where(model2.FieldChatGroupMessageGroupId, groupID).
		Where(model2.FieldChatGroupMessageUserId, userID).
		Where(model2.FieldChatGroupMessageId, messageID)
	msg, err := model2.NewChatGroupMessageModel(repo.db).First(ctx, q)
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query chat message failed: %w", err)

	}

	ret := msg.ToChatGroupMessage()
	return &ret, err
}

type ChatGroupMessageRes struct {
	model2.ChatGroupMessage
	Type string `json:"type"`
}

// GetChatMessages 获取聊天消息列表
func (repo *ChatGroupRepo) GetChatMessages(ctx context.Context, groupID, userID int64, startID, perPage int64) ([]ChatGroupMessageRes, int64, error) {
	q := query.Builder().
		Where(model2.FieldChatGroupMessageGroupId, groupID).
		Where(model2.FieldChatGroupMessageUserId, userID).
		OrderBy(model2.FieldChatGroupMessageId, "DESC").
		Limit(perPage)

	if startID > 0 {
		q = q.Where(model2.FieldChatGroupMessageId, "<", startID)
	}

	messages, err := model2.NewChatGroupMessageModel(repo.db).Get(ctx, q)
	if err != nil {
		return nil, 0, fmt.Errorf("query chat messages failed: %w", err)
	}

	if len(messages) == 0 {
		return []ChatGroupMessageRes{}, startID, nil
	}

	return array.Map(messages, func(message model2.ChatGroupMessageN, _ int) ChatGroupMessageRes {
		ret := message.ToChatGroupMessage()
		if ret.Status == MessageStatusWaiting && ret.CreatedAt.Add(3*time.Minute).Before(time.Now()) {
			// 3 分钟未完成的消息，标记为失败
			ret.Status = MessageStatusFailed
		}
		return ChatGroupMessageRes{
			ChatGroupMessage: ret,
			Type:             ResolveGroupMessageType(ret.Role),
		}
	}), messages[len(messages)-1].Id.ValueOrZero(), nil
}

func ResolveGroupMessageType(role int64) string {
	switch role {
	case 1, 2:
		return "text"
	case 3:
		return "contextBreak"
	case 4:
		return "timeline"
	}

	return "text"
}

func ResolveGroupMessageTypeToRole(messageType string) int64 {
	switch messageType {
	case "contextBreak":
		return 3
	case "timeline":
		return 4
	}

	return 0
}

// DeleteChatMessage 删除聊天消息
func (repo *ChatGroupRepo) DeleteChatMessage(ctx context.Context, groupID, userID, messageID int64) error {
	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().
			Where(model2.FieldChatGroupMessageGroupId, groupID).
			Where(model2.FieldChatGroupMessageUserId, userID).
			Where(model2.FieldChatGroupMessageId, messageID)

		_, err := model2.NewChatGroupMessageModel(tx).Delete(ctx, q)
		return err
	})
}

// DeleteAllChatMessage 清空聊天消息
func (repo *ChatGroupRepo) DeleteAllChatMessage(ctx context.Context, groupID, userID int64) error {
	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().
			Where(model2.FieldChatGroupMessageGroupId, groupID).
			Where(model2.FieldChatGroupMessageUserId, userID)

		_, err := model2.NewChatGroupMessageModel(tx).Delete(ctx, q)
		return err
	})
}

// GetChatMessagesStatus 获取聊天消息状态
func (repo *ChatGroupRepo) GetChatMessagesStatus(ctx context.Context, groupID, userID int64, messageIDs []int64) ([]ChatGroupMessageRes, error) {
	messages, err := model2.NewChatGroupMessageModel(repo.db).Get(ctx, query.Builder().
		Where(model2.FieldChatGroupMessageGroupId, groupID).
		Where(model2.FieldChatGroupMessageUserId, userID).
		WhereIn(model2.FieldChatGroupMessageId, messageIDs))
	if err != nil {
		return nil, fmt.Errorf("query chat messages failed: %w", err)
	}

	return array.Map(messages, func(message model2.ChatGroupMessageN, _ int) ChatGroupMessageRes {
		ret := message.ToChatGroupMessage()
		if ret.Status == MessageStatusWaiting && ret.CreatedAt.Add(3*time.Minute).Before(time.Now()) {
			// 3 分钟未完成的消息，标记为失败
			ret.Status = MessageStatusFailed
		}
		return ChatGroupMessageRes{
			ChatGroupMessage: ret,
			Type:             ResolveGroupMessageType(ret.Role),
		}
	}), nil
}

type ChatGroupMessageUpdate struct {
	Message       string `json:"message,omitempty"`
	TokenConsumed int64  `json:"token_consumed,omitempty"`
	QuotaConsumed int64  `json:"quota_consumed,omitempty"`
	Status        int64  `json:"status,omitempty"`
	Error         string `json:"error,omitempty"`
}

// UpdateChatMessage 更新聊天消息
func (repo *ChatGroupRepo) UpdateChatMessage(ctx context.Context, groupID, userID, messageID int64, msg ChatGroupMessageUpdate) error {
	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		q := query.Builder().
			Where(model2.FieldChatGroupMessageGroupId, groupID).
			Where(model2.FieldChatGroupMessageUserId, userID).
			Where(model2.FieldChatGroupMessageId, messageID)

		_, err := model2.NewChatGroupMessageModel(tx).UpdateFields(ctx, query.KV{
			model2.FieldChatGroupMessageMessage:       msg.Message,
			model2.FieldChatGroupMessageTokenConsumed: msg.TokenConsumed,
			model2.FieldChatGroupMessageQuotaConsumed: msg.QuotaConsumed,
			model2.FieldChatGroupMessageStatus:        msg.Status,
			model2.FieldChatGroupMessageError:         msg.Error,
		}, q)

		return err
	})
}

// DeleteGroup 删除群组
func (repo *ChatGroupRepo) DeleteGroup(ctx context.Context, groupID, userID int64, deleteMessages bool) error {
	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		// 删除历史记录
		if deleteMessages {
			_, err := model2.NewChatGroupMessageModel(tx).Delete(ctx, query.Builder().
				Where(model2.FieldChatGroupMessageGroupId, groupID).
				Where(model2.FieldChatGroupMessageUserId, userID))
			if err != nil {
				return fmt.Errorf("delete chat messages failed: %w", err)
			}
		}

		// 删除成员
		if _, err := model2.NewChatGroupMemberModel(tx).Delete(ctx, query.Builder().
			Where(model2.FieldChatGroupMemberGroupId, groupID).
			Where(model2.FieldChatGroupMemberUserId, userID)); err != nil {
			return fmt.Errorf("delete chat group members failed: %w", err)
		}

		// 删除组
		if _, err := model2.NewRoomsModel(tx).Delete(ctx, query.Builder().
			Where(model2.FieldRoomsId, groupID).
			Where(model2.FieldRoomsUserId, userID)); err != nil {
			return fmt.Errorf("delete chat group failed: %w", err)
		}

		return nil
	})
}
