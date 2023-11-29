package openai

import (
	"context"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
	"net/http"
)

type CompatibleController struct {
	conf *config.Config `autowire:"@"`
}

func NewOpenAICompatibleController(resolver infra.Resolver) web.Controller {
	ctl := &CompatibleController{}
	resolver.MustAutoWire(ctl)
	return ctl
}

func (ctl *CompatibleController) Register(router web.Router) {
	router.Group("/models", func(router web.Router) {
		router.Get("/", ctl.Models)
		router.Get("/{model_id}", ctl.Model)
	})
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func (ctl *CompatibleController) Models(ctx context.Context, webCtx web.Context) web.Response {
	models := array.Map(chat.Models(ctl.conf, false), func(item chat.Model, _ int) Model {
		return Model{
			ID:      item.RealID(),
			Object:  "model",
			Created: 1626777600,
			OwnedBy: item.Category,
		}
	})
	return webCtx.JSON(web.M{"data": models, "object": "list"})
}

func (ctl *CompatibleController) Model(ctx context.Context, webCtx web.Context) web.Response {
	modelID := webCtx.PathVar("model_id")
	matched := array.Filter(chat.Models(ctl.conf, true), func(item chat.Model, _ int) bool {
		return item.RealID() == modelID
	})

	if len(matched) == 0 {
		return webCtx.JSONError("model not found", http.StatusNotFound)
	}

	return webCtx.JSON(Model{
		ID:      modelID,
		Object:  "model",
		Created: 1626777600,
		OwnedBy: matched[0].Category,
	})
}
