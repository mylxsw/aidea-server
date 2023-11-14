package repo

import (
	"context"
	"database/sql"
	"github.com/mylxsw/aidea-server/pkg/repo/model"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/eloquent/query"
	"gopkg.in/guregu/null.v3"
)

const (
	EventTypeUserCreated      = "user_created"
	EventTypeUserPhoneBound   = "user_phone_bound"
	EventTypePaymentCompleted = "payment_completed"
)

type UserCreatedEvent struct {
	UserID int64                  `json:"user_id"`
	From   UserCreatedEventSource `json:"from"`
}

type UserCreatedEventSource string

const (
	UserCreatedEventSourceEmail UserCreatedEventSource = "email"
	UserCreatedEventSourcePhone UserCreatedEventSource = "phone"
)

type UserBindEvent struct {
	UserID int64  `json:"user_id"`
	Phone  string `json:"phone"`
}

type PaymentCompletedEvent struct {
	UserID    int64  `json:"user_id"`
	ProductID string `json:"product_id"`
	PaymentID string `json:"payment_id"`
}

type EventRepo struct {
	db   *sql.DB
	conf *config.Config
}

func NewEventRepo(db *sql.DB, conf *config.Config) *EventRepo {
	return &EventRepo{db: db, conf: conf}
}

func (repo *EventRepo) GetEvent(ctx context.Context, id int64) (*model.Events, error) {
	event, err := model.NewEventsModel(repo.db).First(ctx, query.Builder().Where(model.FieldEventsId, id))
	if err != nil {
		if err == query.ErrNoResult {
			return nil, ErrNotFound
		}

		return nil, err
	}

	ret := event.ToEvents()
	return &ret, nil
}

func (repo *EventRepo) UpdateEvent(ctx context.Context, id int64, status string) error {
	_, err := model.NewEventsModel(repo.db).Update(ctx, query.Builder().Where(model.FieldEventsId, id), model.EventsN{
		Status: null.StringFrom(status),
	})

	return err
}
