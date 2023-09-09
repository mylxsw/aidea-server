package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/mylxsw/aidea-server/internal/repo/model"
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
}

func (r *MessageRepo) Add(ctx context.Context, req MessageAddReq) (int64, error) {
	var id int64
	return id, eloquent.Transaction(r.db, func(tx query.Database) error {
		var err error
		id, err = model.NewChatMessagesModel(tx).Create(ctx, query.KV{
			model.FieldChatMessagesUserId:        req.UserID,
			model.FieldChatMessagesRoomId:        req.RoomID,
			model.FieldChatMessagesRole:          req.Role,
			model.FieldChatMessagesMessage:       req.Message,
			model.FieldChatMessagesQuotaConsumed: req.QuotaConsumed,
			model.FieldChatMessagesTokenConsumed: req.TokenConsumed,
		})
		if err != nil {
			return err
		}

		// 更新房间最后一次操作时间
		if req.RoomID > 1 {
			q := query.Builder().
				Where(model.FieldRoomsUserId, req.UserID).
				Where(model.FieldRoomsId, req.RoomID)

			_, err = model.NewRoomsModel(r.db).Update(ctx, q, model.RoomsN{
				LastActiveTime: null.TimeFrom(time.Now()),
			})
		}

		return err
	})

}
