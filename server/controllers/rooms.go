package controllers

import (
	"context"
	"errors"
	"github.com/mylxsw/aidea-server/pkg/ai/chat"
	"github.com/mylxsw/aidea-server/pkg/misc"
	repo2 "github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

// RoomController 数字人
type RoomController struct {
	roomRepo   *repo2.RoomRepo   `autowire:"@"`
	translater youdao.Translater `autowire:"@"`
	conf       *config.Config    `autowire:"@"`
}

func NewRoomController(resolver infra.Resolver) web.Controller {
	ctl := RoomController{}
	resolver.MustAutoWire(&ctl)

	return &ctl
}

func (ctl *RoomController) Register(router web.Router) {

	router.Group("/rooms", func(router web.Router) {
		router.Post("/", ctl.CreateRoom)
		router.Get("/", ctl.Rooms)
		router.Get("/{room_id}", ctl.Room)
		router.Delete("/{room_id}", ctl.DeleteRoom)
		router.Put("/{room_id}", ctl.UpdateRoom)
		router.Put("/{room_id}/active-time", ctl.UpdateRoomActiveTime)
	})

	router.Group("/room-galleries", func(router web.Router) {
		router.Get("/", ctl.Galleries)
		router.Get("/{id}", ctl.GalleryItem)
		router.Post("/copy", ctl.CopyGalleryItem)
	})
}

const RoomsQueryLimit = 100

// Galleries 系统预置数字人列表
func (ctl *RoomController) Galleries(ctx context.Context, webCtx web.Context, client *auth.ClientInfo) web.Response {
	rooms, err := ctl.roomRepo.Galleries(ctx)
	if err != nil {
		log.Errorf("query rooms galleries failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	cnLocalMode := client.IsCNLocalMode(ctl.conf)
	rooms = array.Filter(rooms, func(item repo2.GalleryRoom, _ int) bool {
		// 如果启用了国产化模式，则过滤掉 openai 和 Anthropic 的模型
		if cnLocalMode && item.RoomType == "system" && array.In(item.Vendor, []string{"openai", "Anthropic"}) {
			return false
		}

		// 检查模型是否满足条件
		if !ctl.conf.EnableOpenAI && item.Vendor == "openai" {
			return false
		}

		if !ctl.conf.EnableBaiduWXAI && item.Vendor == "文心千帆" {
			return false
		}

		if !ctl.conf.EnableDashScopeAI && item.Vendor == "灵积" {
			return false
		}

		if !ctl.conf.EnableXFYunAI && item.Vendor == "讯飞星火" {
			return false
		}

		if !ctl.conf.EnableSenseNovaAI && item.Vendor == "商汤日日新" {
			return false
		}

		if !ctl.conf.EnableTencentAI && item.Vendor == "腾讯" {
			return false
		}

		if !ctl.conf.EnableAnthropic && item.Vendor == "Anthropic" {
			return false
		}

		if !ctl.conf.EnableBaichuan && item.Vendor == "百川" {
			return false
		}

		if !ctl.conf.EnableGPT360 && item.Vendor == "360智脑" {
			return false
		}

		// 检查版本是否满足条件
		if item.VersionMax == "" && item.VersionMin == "" {
			return true
		}

		if client.Version != "" && item.VersionMin != "" && misc.VersionOlder(client.Version, item.VersionMin) {
			return false
		}

		if client.Version != "" && item.VersionMax != "" && misc.VersionNewer(client.Version, item.VersionMax) {
			return false
		}

		return true
	})

	showTags := make([]string, 0)
	for _, item := range rooms {
		showTags = append(showTags, item.Tags...)
	}

	showTags = array.Uniq(showTags)

	tags := []string{"全部", "大模型", "职场", "学习", "娱乐", "世界名人", "创意生活"}
	tags = array.Filter(tags, func(item string, index int) bool {
		return array.In(item, showTags)
	})

	return webCtx.JSON(web.M{
		"data": rooms,
		"tags": tags,
	})
}

// GalleryItem 查询指定数字人详情
func (ctl *RoomController) GalleryItem(ctx context.Context, webCtx web.Context) web.Response {
	id, err := strconv.Atoi(webCtx.PathVar("id"))
	if err != nil {
		return webCtx.JSONError("invalid room id", http.StatusBadRequest)
	}

	room, err := ctl.roomRepo.GalleryItem(ctx, int64(id))
	if err != nil {
		if errors.Is(err, repo2.ErrNotFound) {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, "not found"), http.StatusNotFound)
		}

		log.Errorf("query room gallery item failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data": room,
	})
}

// CopyGalleryItem 用户选择数字人，本地复制一份
func (ctl *RoomController) CopyGalleryItem(ctx context.Context, webCtx web.Context, user *auth.User, client *auth.ClientInfo) web.Response {
	idsStr := strings.Split(webCtx.Input(`ids`), ",")
	ids := array.Filter(
		array.Map(
			idsStr,
			func(s string, _ int) int64 {
				id, _ := strconv.Atoi(strings.TrimSpace(s))
				return int64(id)
			},
		),
		func(id int64, _ int) bool {
			return id > 0
		},
	)
	if len(ids) == 0 {
		return webCtx.JSONError("invalid ids", http.StatusBadRequest)
	}

	// TODO 实时查询，而不是每次全部查询出来再判断是否满足条件，前期内置数字人数量少没关系
	rooms, err := ctl.roomRepo.Galleries(ctx)
	if err != nil {
		log.Errorf("query rooms galleries failed: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	// 启用国产化模式时，如果内置的模型为 GPT 系列，替换为国产模型
	var replaceVendor, replaceModel string
	if client.IsCNLocalMode(ctl.conf) {
		replaceVendor, replaceModel = ctl.conf.CNLocalVendor, ctl.conf.CNLocalModel
	}

	for _, item := range rooms {
		if !array.In(item.Id, ids) {
			continue
		}

		vendor := item.Vendor
		mod := item.Model

		// 如果替换模型和服务商不为空，则替换当前 Room 的模型为国产化模型
		if array.In(strings.ToLower(vendor), []string{"openai", "anthropic"}) && replaceVendor != "" && replaceModel != "" {
			vendor, mod = replaceVendor, replaceModel
		}

		if _, err := ctl.roomRepo.Create(ctx, user.ID, &model.Rooms{
			Name:           item.Name,
			Model:          mod,
			Vendor:         vendor,
			SystemPrompt:   item.Prompt,
			MaxContext:     item.MaxContext,
			RoomType:       repo2.RoomTypePreset,
			InitMessage:    item.InitMessage,
			AvatarId:       item.AvatarId,
			AvatarUrl:      item.AvatarUrl,
			LastActiveTime: time.Now(),
		}, true); err != nil {
			if errors.Is(err, repo2.ErrRoomNameExists) {
				continue
			}

			log.WithFields(log.Fields{
				"room":    item,
				"user_id": user.ID,
			}).Errorf("用户复制数字人失败: %s", err)
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}
	}

	return webCtx.JSON(web.M{})
}

// CreateRoom 创建数字人
func (ctl *RoomController) CreateRoom(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	req, err := ctl.parseRoomRequest(webCtx, false)
	if err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, err.Error()), http.StatusBadRequest)
	}

	if req.MaxContext < 0 || req.MaxContext > 30 {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "最大对话上下文必须为 1-30 之间"), http.StatusBadRequest)
	}

	if req.MaxContext == 0 {
		req.MaxContext = 10
	}

	room := model.Rooms{
		Name:           req.Name,
		UserId:         user.ID,
		Description:    req.Description,
		Model:          req.Model,
		Vendor:         req.Vendor,
		SystemPrompt:   req.SystemPrompt,
		MaxContext:     req.MaxContext,
		RoomType:       repo2.RoomTypeCustom,
		LastActiveTime: time.Now(),
		AvatarId:       req.AvatarID,
		AvatarUrl:      req.AvatarURL,
		InitMessage:    req.InitMessage,
	}

	id, err := ctl.roomRepo.Create(ctx, user.ID, &room, true)
	if err != nil {
		if err == repo2.ErrRoomNameExists {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, "数字人名称已存在"), http.StatusBadRequest)
		}

		log.F(log.M{"user_id": user.ID}).Errorf("创建用户房间失败: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"id": id,
	})
}

// Rooms 获取用户的数字人列表
func (ctl *RoomController) Rooms(ctx context.Context, webCtx web.Context, user *auth.User, client *auth.ClientInfo) web.Response {
	roomTypes := []int{repo2.RoomTypePreset, repo2.RoomTypePresetCustom, repo2.RoomTypeCustom}
	if misc.VersionNewer(client.Version, "1.0.6") {
		roomTypes = append(roomTypes, repo2.RoomTypeGroupChat)
	}

	rooms, err := ctl.roomRepo.Rooms(ctx, user.ID, roomTypes, RoomsQueryLimit)
	if err != nil {
		log.F(log.M{"user_id": user.ID}).Errorf("查询用户房间列表失败: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(rooms)
}

// Room 查询单个数字人信息
func (ctl *RoomController) Room(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	roomID, err := strconv.Atoi(webCtx.PathVar("room_id"))
	if err != nil {
		return webCtx.JSONError("invalid room id", http.StatusBadRequest)
	}

	if roomID == 1 {
		return webCtx.JSON(repo2.GetDefaultRoom())
	}

	room, err := ctl.roomRepo.Room(ctx, user.ID, int64(roomID))
	if err != nil {
		if errors.Is(err, repo2.ErrNotFound) {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, "数字人不存在"), http.StatusNotFound)
		}

		log.F(log.M{"user_id": user.ID, "room_id": roomID}).Errorf("查询用户房间失败: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if room.AvatarUrl == "" {
		for _, mod := range chat.Models(ctl.conf, true) {
			if mod.RealID() == room.Model {
				room.AvatarUrl = mod.AvatarURL
				break
			}
		}
	}

	return webCtx.JSON(room)
}

// DeleteRoom 删除数字人
func (ctl *RoomController) DeleteRoom(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	roomID, err := strconv.Atoi(webCtx.PathVar("room_id"))
	if err != nil {
		return webCtx.JSONError("invalid room id", http.StatusBadRequest)
	}

	if err := ctl.roomRepo.Remove(ctx, user.ID, int64(roomID)); err != nil {
		log.F(log.M{"user_id": user.ID, "room_id": roomID}).Errorf("删除用户房间失败: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

type RoomRequest struct {
	RoomID       int64  `json:"room_id,omitempty"`
	AvatarID     int64  `json:"avatar_id,omitempty"`
	AvatarURL    string `json:"avatar_url,omitempty"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	Model        string `json:"model,omitempty"`
	Vendor       string `json:"vendor,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	InitMessage  string `json:"init_message,omitempty"`
	MaxContext   int64  `json:"max_context,omitempty"`
}

func (ctl *RoomController) parseRoomRequest(webCtx web.Context, isUpdate bool) (*RoomRequest, error) {
	req := RoomRequest{
		MaxContext: webCtx.Int64Input("max_context", 0),
	}

	if req.MaxContext < 0 || req.MaxContext > 30 {
		return nil, errors.New("最大对话上下文必须为 1-30 之间")
	}

	if isUpdate {
		roomID, err := strconv.Atoi(webCtx.PathVar("room_id"))
		if err != nil {
			return nil, errors.New("invalid room id")
		}

		req.RoomID = int64(roomID)
	}

	initMessage := webCtx.Input("init_message")
	if utf8.RuneCountInString(initMessage) > 1000 {
		return nil, errors.New("初始化消息不能超过 1000 个字符")
	}

	req.InitMessage = initMessage

	name := webCtx.Input("name")
	if name == "" {
		return nil, errors.New("数字人名称不能为空")
	}

	if utf8.RuneCountInString(name) > 30 {
		return nil, errors.New("数字人名称不能超过 30 个字符")
	}

	req.Name = name

	description := webCtx.Input("description")
	if utf8.RuneCountInString(description) > 100 {
		return nil, errors.New("数字人描述不能超过 100 个字符")
	}

	req.Description = description

	modelId := webCtx.Input("model")
	if modelId == "" || utf8.RuneCountInString(modelId) > 30 {
		return nil, errors.New("不支持该模型")
	}

	req.Model = modelId

	req.Vendor = webCtx.Input("vendor")
	systemPrompt := webCtx.Input("system_prompt")
	if utf8.RuneCountInString(systemPrompt) > 1000 {
		return nil, errors.New("系统提示不能超过 1000 个字符")
	}

	req.SystemPrompt = systemPrompt

	avatarId := webCtx.Int64Input("avatar_id", 0)
	avatarUrl := webCtx.Input("avatar_url")

	if avatarUrl != "" {
		req.AvatarURL = avatarUrl
	} else {
		req.AvatarID = avatarId
	}

	return &req, nil
}

// UpdateRoom 更新数字人信息
func (ctl *RoomController) UpdateRoom(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	req, err := ctl.parseRoomRequest(webCtx, true)
	if err != nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, err.Error()), http.StatusBadRequest)
	}

	room, err := ctl.roomRepo.Room(ctx, user.ID, int64(req.RoomID))
	if err != nil {
		if errors.Is(err, repo2.ErrNotFound) {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, "数字人不存在"), http.StatusNotFound)
		}

		log.F(log.M{"user_id": user.ID, "room_id": req.RoomID}).Errorf("查询用户房间失败: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	var changed bool

	room.UserId = user.ID
	if req.Name != room.Name {
		room.Name = req.Name
		changed = true
	}

	if req.Description != room.Description {
		room.Description = req.Description
		changed = true
	}

	if req.AvatarID != room.AvatarId && req.AvatarID > 0 {
		room.AvatarId = req.AvatarID
		room.AvatarUrl = ""
		changed = true
	}

	if req.AvatarURL != room.AvatarUrl {
		room.AvatarUrl = req.AvatarURL
		changed = true
	}

	if req.Model != room.Model {
		room.Model = req.Model
		changed = true
	}

	if req.Vendor != room.Vendor {
		room.Vendor = req.Vendor
		changed = true
	}

	if req.SystemPrompt != room.SystemPrompt {
		room.SystemPrompt = req.SystemPrompt
		changed = true
	}

	if req.InitMessage != room.InitMessage {
		room.InitMessage = req.InitMessage
		changed = true
	}

	if req.MaxContext != 0 && req.MaxContext != room.MaxContext {
		if req.MaxContext < 0 || req.MaxContext > 30 {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, "最大对话上下文必须为 1-30 之间"), http.StatusBadRequest)
		}

		room.MaxContext = req.MaxContext
		changed = true
	}

	if changed {
		// 房间内容发生了变化，需要标记为自定义房间
		room.RoomType = repo2.RoomTypePresetCustom
	}

	if err := ctl.roomRepo.Update(ctx, user.ID, req.RoomID, room); err != nil {
		log.F(log.M{"user_id": user.ID, "room_id": req.RoomID}).Errorf("更新用户房间失败: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(room)
}

// UpdateRoomActiveTime 更新数字人活跃时间
func (ctl *RoomController) UpdateRoomActiveTime(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	roomID, err := strconv.Atoi(webCtx.PathVar("room_id"))
	if err != nil {
		return webCtx.JSONError("invalid room id", http.StatusBadRequest)
	}

	if err := ctl.roomRepo.UpdateLastActiveTime(ctx, user.ID, int64(roomID)); err != nil {
		log.F(log.M{"user_id": user.ID, "room_id": roomID}).Errorf("更新用户房间活跃时间失败: %v", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}
