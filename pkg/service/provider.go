package service

import "github.com/mylxsw/glacier/infra"

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(NewUserService)
	binder.MustSingleton(NewSecurityService)
	binder.MustSingleton(NewGalleryService)
	binder.MustSingleton(NewChatService)
}
