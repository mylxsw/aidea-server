package repo

import (
	"errors"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"

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
	binder.MustSingleton(NewChatGroupRepo)

	binder.MustSingleton(func(resolver infra.Resolver) *Repository {
		var repo Repository
		resolver.MustAutoWire(&repo)

		return &repo
	})
}

func (Provider) Boot(resolver infra.Resolver) {
	eventManager := event.NewEventManager(event.NewMemoryEventStore())
	event.SetDispatcher(eventManager)

	resolver.MustResolve(func(conf *config.Config) {
		if !conf.DebugWithSQL {
			return
		}

		eventManager.Listen(func(evt event.QueryExecutedEvent) {
			log.WithFields(log.Fields{
				"sql":      evt.SQL,
				"bindings": evt.Bindings,
				"elapse":   evt.Time.String(),
			}).Debugf("database query executed")
		})
	})
}

type Repository struct {
	Cache     *CacheRepo     `autowire:"@"`
	Quota     *QuotaRepo     `autowire:"@"`
	Queue     *QueueRepo     `autowire:"@"`
	User      *UserRepo      `autowire:"@"`
	Event     *EventRepo     `autowire:"@"`
	Payment   *PaymentRepo   `autowire:"@"`
	Room      *RoomRepo      `autowire:"@"`
	Creative  *CreativeRepo  `autowire:"@"`
	Message   *MessageRepo   `autowire:"@"`
	Prompt    *PromptRepo    `autowire:"@"`
	ChatGroup *ChatGroupRepo `autowire:"@"`
}
