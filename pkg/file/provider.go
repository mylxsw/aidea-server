package file

import (
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(up *uploader.Uploader, cache *repo.CacheRepo) *File {
		return New(up, cache)
	})
}
