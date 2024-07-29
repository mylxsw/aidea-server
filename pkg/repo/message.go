package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/must"
	"strings"
	"time"

	"github.com/mylxsw/eloquent"
	"github.com/mylxsw/eloquent/query"
	"gopkg.in/guregu/null.v3"
)

type MessageRepo struct {
	db *sql.DB
}

func NewMessageRepo(db *sql.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

type MessageRole int64

const (
	MessageRoleUser      MessageRole = 1
	MessageRoleAssistant MessageRole = 2
)

type MessageAddReq struct {
	UserID        int64
	RoomID        int64
	Role          MessageRole
	Message       string
	QuotaConsumed int64
	TokenConsumed int64
	PID           int64
	Model         string
	Status        int64
	Error         string
}

func (r *MessageRepo) Add(ctx context.Context, req MessageAddReq) (int64, error) {
	if req.Status == 0 {
		req.Status = MessageStatusSucceed
	}

	var id int64
	kvs := query.KV{
		model.FieldChatMessagesUserId:        req.UserID,
		model.FieldChatMessagesRoomId:        req.RoomID,
		model.FieldChatMessagesRole:          req.Role,
		model.FieldChatMessagesMessage:       req.Message,
		model.FieldChatMessagesQuotaConsumed: req.QuotaConsumed,
		model.FieldChatMessagesTokenConsumed: req.TokenConsumed,
		model.FieldChatMessagesStatus:        req.Status,
	}

	if req.PID > 0 {
		kvs[model.FieldChatMessagesPid] = req.PID
	}

	if req.Model != "" {
		kvs[model.FieldChatMessagesModel] = req.Model
	}

	if req.Error != "" {
		kvs[model.FieldChatMessagesError] = req.Error
	}

	return id, eloquent.Transaction(r.db, func(tx query.Database) error {
		var err error
		id, err = model.NewChatMessagesModel(tx).Create(ctx, kvs)
		if err != nil {
			return err
		}

		// 更新房间最后一次操作时间
		if req.RoomID > 1 && req.Role == MessageRoleUser {
			q := query.Builder().
				Where(model.FieldRoomsUserId, req.UserID).
				Where(model.FieldRoomsId, req.RoomID)

			_, err = model.NewRoomsModel(r.db).Update(ctx, q, model.RoomsN{
				LastActiveTime: null.TimeFrom(time.Now()),
				Description:    null.StringFrom(misc.SubString(req.Message, 70)),
			})
		}

		return err
	})

}

type MessageUpdateReq struct {
	Status int64
	Error  string
}

func (r *MessageRepo) UpdateMessageStatus(ctx context.Context, id int64, req MessageUpdateReq) error {
	kv := query.KV{}

	if req.Status > 0 {
		kv[model.FieldChatMessagesStatus] = req.Status
	}

	if req.Error != "" {
		kv[model.FieldChatMessagesError] = req.Error
	}

	if len(kv) == 0 {
		return nil
	}

	_, err := model.NewChatMessagesModel(r.db).UpdateFields(ctx, kv, query.Builder().Where(model.FieldChatMessagesId, id))
	return err
}

func (r *MessageRepo) RecentlyMessages(ctx context.Context, userID, roomID int64, offset, limit int64) ([]model.ChatMessages, error) {
	q := query.Builder().
		OrderBy(model.FieldChatMessagesId, "DESC").
		Offset(offset).
		Limit(limit)

	if userID > 0 {
		q = q.Where(model.FieldChatMessagesUserId, userID)
	}

	if roomID > 0 {
		q = q.Where(model.FieldChatMessagesRoomId, roomID)
	}

	messages, err := model.NewChatMessagesModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	return array.Map(messages, func(m model.ChatMessagesN, _ int) model.ChatMessages { return m.ToChatMessages() }), nil
}

func (r *MessageRepo) Messages(ctx context.Context, page, perPage int64, options ...QueryOption) ([]model.ChatMessages, query.PaginateMeta, error) {
	q := query.Builder().OrderBy(model.FieldChatMessagesId, "DESC")
	for _, opt := range options {
		q = opt(q)
	}

	messages, meta, err := model.NewChatMessagesModel(r.db).Paginate(ctx, page, perPage, q)
	if err != nil {
		return nil, query.PaginateMeta{}, err
	}

	return array.Map(messages, func(item model.ChatMessagesN, _ int) model.ChatMessages {
		return item.ToChatMessages()
	}), meta, nil
}

type ShareData struct {
	IDs   []int64 `json:"ids"`
	Style string  `json:"style,omitempty"`
}

func (data ShareData) String() string {
	ids := array.Sort(data.IDs, func(a, b int64) bool { return a < b })
	return fmt.Sprintf(
		"%s:%s",
		strings.Join(array.Map(ids, func(id int64, _ int) string { return fmt.Sprintf("%d", id) }), ","),
		data.Style,
	)
}

// Share chat history, generate a sharing code
func (r *MessageRepo) Share(ctx context.Context, userID int64, data ShareData) (string, error) {
	if len(data.IDs) == 0 {
		return "", errors.New("no message ids")
	}

	shareCode := misc.Sha1([]byte(fmt.Sprintf("%d:%s", userID, data.String())))

	shareInfo, err := model.NewChatMessagesShareModel(r.db).First(ctx, query.Builder().Where(model.FieldChatMessagesShareCode, shareCode))
	if err == nil && shareInfo != nil {
		return shareCode, nil
	}

	if _, err := model.NewChatMessagesShareModel(r.db).Create(ctx, query.KV{
		model.FieldChatMessagesShareCode:   shareCode,
		model.FieldChatMessagesShareUserId: userID,
		model.FieldChatMessagesShareData:   string(must.Must(json.Marshal(data))),
	}); err != nil {
		return "", err
	}

	return shareCode, nil
}

// SharedMessages get shared messages by sharing code
func (r *MessageRepo) SharedMessages(ctx context.Context, code string) ([]model.ChatMessages, *ShareData, error) {
	shareInfo, err := model.NewChatMessagesShareModel(r.db).First(ctx, query.Builder().Where(model.FieldChatMessagesShareCode, code))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, nil, ErrNotFound
		}

		return nil, nil, err
	}

	var data ShareData
	if err := json.Unmarshal([]byte(shareInfo.Data.ValueOrZero()), &data); err != nil {
		return nil, nil, err
	}

	q := query.Builder().
		WhereIn(model.FieldChatMessagesId, data.IDs).
		Where(model.FieldChatMessagesUserId, shareInfo.UserId.ValueOrZero())
	messages, err := model.NewChatMessagesModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, nil, err
	}

	if len(messages) == 0 {
		return nil, nil, ErrNotFound
	}

	msgs := array.Map(messages, func(m model.ChatMessagesN, _ int) model.ChatMessages { return m.ToChatMessages() })
	return msgs, &data, nil
}
