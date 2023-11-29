package migrate

import (
	"context"
	"database/sql"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {}

func (Provider) Boot(resolver infra.Resolver) {
	resolver.MustResolve(func(db *sql.DB) {
		if err := Migrate(context.TODO(), db); err != nil {
			log.Errorf("migrate database failed: %v", err)
		}
	})
}

func (Provider) ShouldLoad(c infra.FlagContext) bool {
	return c.Bool("enable-migrate")
}
