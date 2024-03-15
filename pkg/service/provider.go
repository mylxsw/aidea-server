package service

import "github.com/mylxsw/glacier/infra"

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(NewUserService)
	binder.MustSingleton(NewSecurityService)
	binder.MustSingleton(NewGalleryService)
	binder.MustSingleton(NewChatService)

	binder.MustSingleton(func(resolver infra.Resolver) *Service {
		var svc Service
		resolver.MustAutoWire(&svc)

		return &svc
	})
}

type Service struct {
	User     *UserService     `autowire:"@"`
	Security *SecurityService `autowire:"@"`
	Gallery  *GalleryService  `autowire:"@"`
	Chat     *ChatService     `autowire:"@"`
}
