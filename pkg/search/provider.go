package search

import (
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) *BigModelSearch {
		return NewBigModelSearch(conf.BigModelSearchAPIKey)
	})

	binder.MustSingleton(func(conf *config.Config) Searcher {
		return NewSearcher(conf)
	})
}
