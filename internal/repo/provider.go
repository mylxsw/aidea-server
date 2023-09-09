package repo

import (
	"errors"

	"github.com/mylxsw/eloquent/event"
	"github.com/mylxsw/glacier/infra"
)

var (
	ErrNotFound = errors.New("not found")
)

const (
	EventStatusWaiting = "waiting"
	EventStatusSucceed = "succeed"
	EventStatusFailed  = "failed"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(NewQuotaRepo)
	binder.MustSingleton(NewCacheRepo)
	binder.MustSingleton(NewQueueRepo)
	binder.MustSingleton(NewUserRepo)
	binder.MustSingleton(NewEventRepo)
	binder.MustSingleton(NewPaymentRepo)
	binder.MustSingleton(NewRoomRepo)
	binder.MustSingleton(NewCreativeRepo)
	binder.MustSingleton(NewMessageRepo)
	binder.MustSingleton(NewPromptRepo)
}

func (Provider) Boot(resolver infra.Resolver) {
	eventManager := event.NewEventManager(event.NewMemoryEventStore())
	event.SetDispatcher(eventManager)

	// eventManager.Listen(func(evt event.QueryExecutedEvent) {
	// 	log.WithFields(log.Fields{
	// 		"sql":      evt.SQL,
	// 		"bindings": evt.Bindings,
	// 		"elapse":   evt.Time.String(),
	// 	}).Debugf("database query executed")
	// })
}
