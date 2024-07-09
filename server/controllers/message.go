package controllers

import (
	"context"
	"errors"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
	"net/http"
)

type MessageController struct {
	repo *repo.Repository `autowire:"@"`
}

func NewMessageController(resolver infra.Resolver) web.Controller {
	ctl := MessageController{}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *MessageController) Register(router web.Router) {
	router.Group("/messages", func(router web.Router) {
		router.Post("/share", ctl.ShareMessages)
	})

	router.Group("/shared-messages", func(router web.Router) {
		router.Get("/{code}", ctl.GetSharedMessages)
	})
}

type MessageShareRequest struct {
	IDs []int64 `json:"ids"`
}

type MessageShareResponse struct {
	Code string `json:"code"`
}

// ShareMessages Share messages to other users
// @Summary Share messages to other users
// @Tags Message
// @Accept json
// @Produce json
// @Param req body MessageShareRequest true "Message Share Request"
// @Success 200 {object} MessageShareResponse
// @Router /v1/messages/share [post]
func (ctl *MessageController) ShareMessages(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	var req MessageShareRequest
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	req.IDs = array.Uniq(array.Filter(req.IDs, func(id int64, _ int) bool { return id > 0 }))
	if len(req.IDs) == 0 {
		return webCtx.JSONError("invalid message ids", http.StatusBadRequest)
	}

	code, err := ctl.repo.Message.Share(ctx, user.ID, req.IDs)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(MessageShareResponse{Code: code})
}

// GetSharedMessages Get shared messages by code
// @Summary Get shared messages by code
// @Tags Message
// @Accept json
// @Produce json
// @Param code path string true "Share Code"
// @Success 200 {object} common.DataArray[model.ChatMessages]
// @Router /v1/shared-messages/{code} [get]
func (ctl *MessageController) GetSharedMessages(ctx context.Context, webCtx web.Context) web.Response {
	code := webCtx.PathVar("code")
	if code == "" {
		return webCtx.JSONError("invalid code", http.StatusBadRequest)
	}

	messages, err := ctl.repo.Message.SharedMessages(ctx, code)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return webCtx.JSONError(err.Error(), http.StatusNotFound)
		}

		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(common.NewDataArray(messages))
}
