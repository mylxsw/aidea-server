package repo

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/go-utils/array"
)

type NotificationRepo struct {
	db   *sql.DB
	conf *config.Config
}

// NewNotificationRepo create a new NotificationRepo
func NewNotificationRepo(db *sql.DB, conf *config.Config) *NotificationRepo {
	return &NotificationRepo{db: db, conf: conf}
}

// NotifyMessages 获取通知消息列表
func (repo *NotificationRepo) NotifyMessages(ctx context.Context, startID, perPage int64) ([]model.Notifications, int64, error) {
	q := query.Builder().
		OrderBy(model.FieldNotificationsId, "DESC").
		Limit(perPage)

	if startID > 0 {
		q = q.Where(model.FieldNotificationsId, "<", startID)
	}

	messages, err := model.NewNotificationsModel(repo.db).Get(ctx, q)
	if err != nil {
		return nil, 0, fmt.Errorf("query chat messages failed: %w", err)
	}

	if len(messages) == 0 {
		return []model.Notifications{}, startID, nil
	}

	return array.Map(messages, func(message model.NotificationsN, _ int) model.Notifications {
		return message.ToNotifications()
	}), messages[len(messages)-1].Id.ValueOrZero(), nil
}
