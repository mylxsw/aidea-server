package admin

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/str"
	"net/http"
	"strconv"
)

type ChannelController struct {
	repo *repo.Repository `autowire:"@"`
	svc  *service.Service `autowire:"@"`
}

func NewChannelController(resolver infra.Resolver) web.Controller {
	ctl := &ChannelController{}
	resolver.MustAutoWire(ctl)

	return ctl
}

func (ctl *ChannelController) Register(router web.Router) {
	router.Group("/channels", func(router web.Router) {
		router.Get("/", ctl.Channels)
		router.Post("/", ctl.Add)
		router.Get("/{channel_id}", ctl.Channel)
		router.Put("/{channel_id}", ctl.Update)
		router.Delete("/{channel_id}", ctl.Delete)
	})

	router.Group("/channel-types", func(router web.Router) {
		router.Get("/", ctl.ChannelTypes)
	})
}

// ChannelTypes 返回所有的渠道类型列表
func (ctl *ChannelController) ChannelTypes(ctx context.Context, webCtx web.Context) web.Response {
	return webCtx.JSON(web.M{
		"data": ctl.svc.Chat.ChannelTypes(),
	})
}

type Channel struct {
	repo.Channel
	DisplayName string `json:"display_name,omitempty"`
}

// Channels 返回所有的渠道列表
func (ctl *ChannelController) Channels(ctx context.Context, webCtx web.Context) web.Response {
	channels, err := ctl.repo.Model.GetChannels(ctx)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	types := array.ToMap(ctl.svc.Chat.ChannelTypes(), func(t service.ChannelType, _ int) string {
		return t.Name
	})

	data := array.Map(channels, func(item repo.Channel, _ int) Channel {
		item.Secret = ""
		ret := Channel{Channel: item}
		if ret.Id == 0 {
			ret.DisplayName = types[item.Name].Display
		}

		return ret
	})

	return webCtx.JSON(web.M{
		"data": data,
	})
}

// Channel 返回指定渠道的详细信息
func (ctl *ChannelController) Channel(ctx context.Context, webCtx web.Context) web.Response {
	channelID, err := strconv.Atoi(webCtx.PathVar("channel_id"))
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	channel, err := ctl.repo.Model.GetChannel(ctx, int64(channelID))
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	data := Channel{Channel: *channel}
	if data.Id == 0 {
		types := array.ToMap(ctl.svc.Chat.ChannelTypes(), func(t service.ChannelType, _ int) string {
			return t.Name
		})
		data.DisplayName = types[channel.Name].Display
	}

	return webCtx.JSON(web.M{"data": channel})
}

// Add 添加渠道
func (ctl *ChannelController) Add(ctx context.Context, webCtx web.Context) web.Response {
	var req repo.ChannelAddReq
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	allowTypes := array.Map(
		array.Filter(ctl.svc.Chat.ChannelTypes(), func(item service.ChannelType, _ int) bool { return item.Dynamic }),
		func(item service.ChannelType, _ int) string { return item.Name },
	)

	if !array.In(req.Type, allowTypes) {
		return webCtx.JSONError("不支持该渠道类型", http.StatusBadRequest)
	}

	if req.Name == "" {
		return webCtx.JSONError("渠道名称不能为空", http.StatusBadRequest)
	}

	if !str.HasPrefixes(req.Server, []string{"http://", "https://"}) {
		return webCtx.JSONError("服务器地址不合法", http.StatusBadRequest)
	}

	channelID, err := ctl.repo.Model.AddChannel(ctx, req)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"channel_id": channelID})
}

// Update 更新渠道信息
func (ctl *ChannelController) Update(ctx context.Context, webCtx web.Context) web.Response {
	channelID, err := strconv.Atoi(webCtx.PathVar("channel_id"))
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	var req repo.ChannelUpdateReq
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	if req.Name == "" {
		return webCtx.JSONError("渠道名称不能为空", http.StatusBadRequest)
	}

	if !str.HasPrefixes(req.Server, []string{"http://", "https://"}) {
		return webCtx.JSONError("服务器地址不合法", http.StatusBadRequest)
	}

	if err := ctl.repo.Model.UpdateChannel(ctx, int64(channelID), req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// Delete 删除渠道
func (ctl *ChannelController) Delete(ctx context.Context, webCtx web.Context) web.Response {
	channelID, err := strconv.Atoi(webCtx.PathVar("channel_id"))
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	if err := ctl.repo.Model.DeleteChannel(ctx, int64(channelID)); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}
