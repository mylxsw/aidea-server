package repo

import (
	"context"
	"database/sql"
	"github.com/mylxsw/aidea-server/pkg/misc"
	model2 "github.com/mylxsw/aidea-server/pkg/repo/model"
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
	RoomID  int64
	Role    MessageRole
	Message string
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
		model2.FieldChatMessagesUserId:        req.UserID,
		model2.FieldChatMessagesRoomId:        req.RoomID,
		model2.FieldChatMessagesRole:          req.Role,
		model2.FieldChatMessagesMessage:       req.Message,
		model2.FieldChatMessagesQuotaConsumed: req.QuotaConsumed,
		model2.FieldChatMessagesTokenConsumed: req.TokenConsumed,
		model2.FieldChatMessagesStatus:        req.Status,
	}

	if req.PID > 0 {
		kvs[model2.FieldChatMessagesPid] = req.PID
	}

	if req.Model != "" {
		kvs[model2.FieldChatMessagesModel] = req.Model
	}

	if req.Error != "" {
		kvs[model2.FieldChatMessagesError] = req.Error
	}

	return id, eloquent.Transaction(r.db, func(tx query.Database) error {
		var err error
		id, err = model2.NewChatMessagesModel(tx).Create(ctx, kvs)
		if err != nil {
			return err
		}

		// 更新房间最后一次操作时间
		if req.RoomID > 1 && req.Role == MessageRoleUser {
			q := query.Builder().
				Where(model2.FieldRoomsUserId, req.UserID).
				Where(model2.FieldRoomsId, req.RoomID)

			_, err = model2.NewRoomsModel(r.db).Update(ctx, q, model2.RoomsN{
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
		kv[model2.FieldChatMessagesStatus] = req.Status
	}

	if req.Error != "" {
		kv[model2.FieldChatMessagesError] = req.Error
	}

	if len(kv) == 0 {
		return nil
	}

	_, err := model2.NewChatMessagesModel(r.db).UpdateFields(ctx, kv, query.Builder().Where(model2.FieldChatMessagesId, id))
	return err
}
