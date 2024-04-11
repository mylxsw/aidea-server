package admin

import (
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
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
		router.Get("/{user_id}/rooms/{room_id}/group-messages", ctl.GroupRoomMessages)
	})

	router.Group("/recent-messages", func(router web.Router) {
		router.Get("/", ctl.RecentMessages)
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

	rooms = append([]repo.Room{{Rooms: *repo.GetDefaultRoom()}}, rooms...)
	models := array.ToMap(
		ctl.svc.Chat.Models(ctx, true),
		func(item repo.Model, _ int) string { return item.ModelId },
	)

	return ctx.JSON(common.NewDataArray(array.Map(rooms, func(item repo.Room, _ int) repo.Room {
		// 替换成员列表为头像列表
		members := make([]string, 0)
		if len(item.Members) > 0 {
			for _, member := range item.Members {
				if mod, ok := models[member]; ok && mod.AvatarUrl != "" {
					members = append(members, mod.AvatarUrl)
				}
			}

			item.Members = members
		}

		if item.AvatarUrl == "" {
			if mod, ok := models[item.Model]; ok && mod.AvatarUrl != "" {
				item.AvatarUrl = mod.AvatarUrl
			}
		}

		return item
	})))
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

type ChatGroupMessage struct {
	repo.ChatGroupMessageRes
	Model     string `json:"model,omitempty"`
	ModelName string `json:"model_name,omitempty"`
}

// GroupRoomMessages Get the latest 200 messages in the specified group chat room for the specified user.
// @Summary Get the latest 200 messages in the specified group chat room for the specified user.
// @Tags Admin:Messages
// @Produce json
// @Param user_id path integer true "User ID"
// @Param room_id path integer true "Room ID"
// @Success 200 {object} common.DataArray[ChatGroupMessage]
// @Router /v1/admin/messages/{user_id}/rooms/{room_id}/group-messages [get]
func (ctl *MessageController) GroupRoomMessages(ctx web.Context) web.Response {
	userID, err := strconv.Atoi(ctx.PathVar("user_id"))
	if err != nil {
		return ctx.JSONError("invalid user_id", http.StatusBadRequest)
	}

	roomID, err := strconv.Atoi(ctx.PathVar("room_id"))
	if err != nil {
		return ctx.JSONError("invalid room_id", http.StatusBadRequest)
	}

	messages, _, err := ctl.repo.ChatGroup.GetChatMessages(ctx, int64(roomID), int64(userID), 0, 200)
	if err != nil {
		return ctx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	// TODO...
	memberIds := array.Uniq(array.Map(messages, func(item repo.ChatGroupMessageRes, _ int) int64 {
		return item.MemberId
	}))

	members, err := ctl.repo.ChatGroup.GetMembers(ctx, memberIds)
	if err == nil {
		membersMap := array.ToMap(members, func(t repo.Member, _ int) int { return t.ID })
		return ctx.JSON(common.NewDataArray(array.Map(messages, func(item repo.ChatGroupMessageRes, _ int) ChatGroupMessage {
			return ChatGroupMessage{
				ChatGroupMessageRes: item,
				Model:               membersMap[int(item.MemberId)].ModelID,
				ModelName:           membersMap[int(item.MemberId)].ModelID,
			}
		})))
	}

	return ctx.JSON(common.NewDataArray(messages))
}

// RecentMessages Get the latest 20 messages.
// @Summary Get the latest 20 messages.
// @Tags Admin:Messages
// @Produce json
// @Param page query integer false "Page number" default(1)
// @Param per_page query integer false "Number of items per page" default(20)
// @Param keyword query string false "Support searching by message content and model name (fuzzy matching)"
// @Success 200 {object} common.Pagination[model.ChatMessages]
// @Router /v1/admin/recent-messages [get]
func (ctl *MessageController) RecentMessages(ctx web.Context) web.Response {
	page := ctx.Int64Input("page", 1)
	if page < 1 || page > 1000 {
		page = 1
	}

	perPage := ctx.Int64Input("per_page", 20)
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	keyword := ctx.Input("keyword")
	opt := func(builder query.SQLBuilder) query.SQLBuilder {
		if keyword != "" {
			builder = builder.WhereGroup(func(builder query.Condition) {
				builder.Where(model.FieldChatMessagesMessage, query.LIKE, "%"+keyword+"%").
					OrWhere(model.FieldChatMessagesModel, query.LIKE, "%"+keyword+"%")
			})
		}

		return builder.Where(model.FieldChatMessagesRole, repo.MessageRoleUser)
	}

	items, meta, err := ctl.repo.Message.Messages(ctx, page, perPage, opt)
	if err != nil {
		return ctx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return ctx.JSON(common.NewPagination(items, meta))
}
