package controllers

import (
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
)

type MessageController struct {
	// repo *repo.Repository `autowire:"@"`
	// conf *config.Config   `autowire:"@"`
}

func NewMessageController(resolver infra.Resolver) web.Controller {
	ctl := MessageController{}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *MessageController) Register(router web.Router) {
	router.Group("/messages", func(router web.Router) {

	})
}
