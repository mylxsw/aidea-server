package admin

import (
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"net/http"
	"strconv"
)

type MessageController struct {
	svc  *service.Service `autowire:"@"`
	repo *repo.Repository `autowire:"@"`
}

func NewMessageController(resolver infra.Resolver) web.Controller {
	return infra.Autowire(resolver, &MessageController{})
}

func (ctl *MessageController) Register(router web.Router) {
	router.Group("/messages", func(router web.Router) {
		router.Get("/{user_id}/rooms", ctl.UserRooms)
		router.Get("/{user_id}/rooms/{room_id}", ctl.UserRoom)
		router.Get("/{user_id}/rooms/{room_id}/messages", ctl.RoomMessages)
	})
}

// UserRooms Get a list of all chat rooms for the specified user.
// @Summary Get a list of all chat rooms for the specified user.
// @Tags Admin:Messages
// @Produce json
// @Param user_id path integer true "User ID"
// @Success 200 {object} common.DataArray[repo.Room]
// @Router /v1/admin/messages/{user_id}/rooms [get]
func (ctl *MessageController) UserRooms(ctx web.Context) web.Response {
	userID, err := strconv.Atoi(ctx.PathVar("user_id"))
	if err != nil {
		return ctx.JSONError("invalid user_id", http.StatusBadRequest)
	}

	roomTypes := []int{repo.RoomTypePreset, repo.RoomTypePresetCustom, repo.RoomTypeCustom, repo.RoomTypeGroupChat}
	rooms, err := ctl.repo.Room.Rooms(ctx, int64(userID), roomTypes, 200)
	if err != nil {
		return ctx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	rooms = append(rooms, repo.Room{Rooms: *repo.GetDefaultRoom()})

	return ctx.JSON(common.NewDataArray(rooms))
}

// UserRoom Get the specified chat room information for the specified user.
// @Summary Get the specified chat room information for the specified user.
// @Tags Admin:Messages
// @Produce json
// @Param user_id path integer true "User ID"
// @Param room_id path integer true "Room ID"
// @Success 200 {object} common.DataObj[repo.Room]
// @Router /v1/admin/messages/{user_id}/rooms/{room_id} [get]
func (ctl *MessageController) UserRoom(ctx web.Context) web.Response {
	userID, err := strconv.Atoi(ctx.PathVar("user_id"))
	if err != nil {
		return ctx.JSONError("invalid user_id", http.StatusBadRequest)
	}

	roomID, err := strconv.Atoi(ctx.PathVar("room_id"))
	if err != nil {
		return ctx.JSONError("invalid room_id", http.StatusBadRequest)
	}

	room, err := ctl.repo.Room.Room(ctx, int64(userID), int64(roomID))
	if err != nil {
		return ctx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return ctx.JSON(common.NewDataObj(room))
}

// RoomMessages Get the latest 200 messages in the specified chat room for the specified user.
// @Summary Get the latest 200 messages in the specified chat room for the specified user.
// @Tags Admin:Messages
// @Produce json
// @Param user_id path integer true "User ID"
// @Param room_id path integer true "Room ID"
// @Success 200 {object} common.DataArray[model.ChatMessages]
// @Router /v1/admin/messages/{user_id}/rooms/{room_id}/messages [get]
func (ctl *MessageController) RoomMessages(ctx web.Context) web.Response {
	userID, err := strconv.Atoi(ctx.PathVar("user_id"))
	if err != nil {
		return ctx.JSONError("invalid user_id", http.StatusBadRequest)
	}

	roomID, err := strconv.Atoi(ctx.PathVar("room_id"))
	if err != nil {
		return ctx.JSONError("invalid room_id", http.StatusBadRequest)
	}

	messages, err := ctl.repo.Message.RecentlyMessages(ctx, int64(userID), int64(roomID), 0, 200)
	if err != nil {
		return ctx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return ctx.JSON(common.NewDataArray(messages))
}
