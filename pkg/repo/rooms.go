package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	model2 "github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/go-utils/maps"
	"time"

	"github.com/mylxsw/asteria/log"

	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/go-utils/array"
	"gopkg.in/guregu/null.v3"
)

var (
	ErrRoomNameExists = errors.New("room name exists")
)

const (
	// RoomTypePreset 预设房间
	RoomTypePreset = 1
	// RoomTypeCustom 自定义房间
	RoomTypeCustom = 2
	// RoomTypePresetCustom 预设被用户修改过
	RoomTypePresetCustom = 3
	// RoomTypeGroupChat 群聊
	RoomTypeGroupChat = 4
)

type RoomRepo struct {
	db *sql.DB
}

func NewRoomRepo(db *sql.DB) *RoomRepo {
	return &RoomRepo{db: db}
}

type Room struct {
	model2.Rooms
	Members []string `json:"members,omitempty"`
}

func (r *RoomRepo) Rooms(ctx context.Context, userID int64, roomTypes []int, limit int64) ([]Room, error) {
	q := query.Builder().
		Where(model2.FieldRoomsUserId, userID).
		WhereIn(model2.FieldRoomsRoomType, roomTypes).
		OrderBy(model2.FieldRoomsPriority, "DESC").
		OrderBy(model2.FieldRoomsLastActiveTime, "DESC").
		Limit(limit)

	rooms, err := model2.NewRoomsModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	// 查询群聊头像列表
	groupRooms := array.Filter(rooms, func(item model2.RoomsN, index int) bool {
		return item.RoomType.ValueOrZero() == RoomTypeGroupChat
	})

	var groupMembers map[int64][]string
	if len(groupRooms) > 0 {
		groupIDs := array.Map(groupRooms, func(item model2.RoomsN, index int) int64 { return item.Id.ValueOrZero() })

		q := query.Builder().
			WhereIn(model2.FieldChatGroupMemberGroupId, groupIDs).
			Where(model2.FieldChatGroupMemberUserId, userID).
			Where(model2.FieldChatGroupMemberStatus, MessageStatusSucceed)

		members, err := model2.NewChatGroupMemberModel(r.db).Get(ctx, q)
		if err != nil {
			log.Errorf("query chat group members failed: %v", err)
		}

		groupMembers = maps.Map(
			array.GroupBy(members, func(item model2.ChatGroupMemberN) int64 { return item.GroupId.ValueOrZero() }),
			func(items []model2.ChatGroupMemberN, _ int64) []string {
				return array.Map(items, func(item model2.ChatGroupMemberN, _ int) string { return item.ModelId.ValueOrZero() })
			},
		)
	}

	return array.Map(rooms, func(room model2.RoomsN, _ int) Room {
		return Room{
			Rooms:   room.ToRooms(),
			Members: groupMembers[room.Id.ValueOrZero()],
		}
	}), nil
}

func GetDefaultRoom() *model2.Rooms {
	return &model2.Rooms{
		Id:         1,
		Name:       "默认",
		Model:      "gpt-3.5-turbo",
		Vendor:     "openai",
		MaxContext: 5,
		RoomType:   RoomTypePreset,
	}
}

func (r *RoomRepo) Room(ctx context.Context, userID, roomID int64) (*model2.Rooms, error) {
	if roomID == 1 {
		return GetDefaultRoom(), nil
	}

	q := query.Builder().
		Where(model2.FieldRoomsUserId, userID).
		Where(model2.FieldRoomsId, roomID)

	room, err := model2.NewRoomsModel(r.db).First(ctx, q)
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	ret := room.ToRooms()
	return &ret, nil
}

func (r *RoomRepo) Create(ctx context.Context, userID int64, room *model2.Rooms, enableDup bool) (id int64, err error) {
	if !enableDup {
		q := query.Builder().
			Where(model2.FieldRoomsName, room.Name).
			Where(model2.FieldRoomsUserId, userID)
		exist, err := model2.NewRoomsModel(r.db).Exists(ctx, q)
		if err != nil {
			return 0, err
		}

		if exist {
			return 0, ErrRoomNameExists
		}
	}

	room.UserId = userID

	roomN := room.ToRoomsN(
		model2.FieldRoomsName,
		model2.FieldRoomsUserId,
		model2.FieldRoomsAvatarId,
		model2.FieldRoomsAvatarUrl,
		model2.FieldRoomsDescription,
		model2.FieldRoomsPriority,
		model2.FieldRoomsModel,
		model2.FieldRoomsVendor,
		model2.FieldRoomsSystemPrompt,
		model2.FieldRoomsLastActiveTime,
		model2.FieldRoomsMaxContext,
		model2.FieldRoomsRoomType,
		model2.FieldRoomsInitMessage,
	)

	id, err = model2.NewRoomsModel(r.db).Save(ctx, roomN)

	return
}

func (r *RoomRepo) Remove(ctx context.Context, userID, roomID int64) error {
	q := query.Builder().
		Where(model2.FieldRoomsUserId, userID).
		Where(model2.FieldRoomsId, roomID)

	_, err := model2.NewRoomsModel(r.db).Delete(ctx, q)
	return err
}

func (r *RoomRepo) Update(ctx context.Context, userID, roomID int64, room *model2.Rooms) error {
	q := query.Builder().
		Where(model2.FieldRoomsUserId, userID).
		Where(model2.FieldRoomsId, roomID)

	_, err := model2.NewRoomsModel(r.db).Update(ctx, q, room.ToRoomsN(
		model2.FieldRoomsName,
		model2.FieldRoomsDescription,
		model2.FieldRoomsAvatarId,
		model2.FieldRoomsAvatarUrl,
		model2.FieldRoomsPriority,
		model2.FieldRoomsModel,
		model2.FieldRoomsVendor,
		model2.FieldRoomsSystemPrompt,
		model2.FieldRoomsMaxContext,
		model2.FieldRoomsRoomType,
		model2.FieldRoomsInitMessage,
	))

	return err
}

func (r *RoomRepo) UpdateLastActiveTime(ctx context.Context, userID, roomID int64) error {
	q := query.Builder().
		Where(model2.FieldRoomsUserId, userID).
		Where(model2.FieldRoomsId, roomID)

	_, err := model2.NewRoomsModel(r.db).Update(ctx, q, model2.RoomsN{
		LastActiveTime: null.TimeFrom(time.Now()),
	})

	return err
}

type GalleryRoom struct {
	Id          int64    `json:"id"`
	Name        string   `json:"name,omitempty"`
	AvatarUrl   string   `json:"avatar_url,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	VersionMin  string   `json:"-"`
	VersionMax  string   `json:"-"`
	RoomType    string   `json:"-"`

	AvatarId    int64  `json:"-"`
	Model       string `json:"-"`
	Vendor      string `json:"-"`
	Prompt      string `json:"-"`
	MaxContext  int64  `json:"-"`
	InitMessage string `json:"-"`
}

func createGalleryRoomFromModel(room model2.RoomGallery) GalleryRoom {
	var tags []string
	if err := json.Unmarshal([]byte(room.Tags), &tags); err != nil {
		tags = []string{}
		log.WithFields(log.Fields{"room": room}).Errorf("unmarshal room gallery tags failed, err: %v", err)
	}

	description := room.Description
	if description == "" {
		description = room.Prompt
	}

	return GalleryRoom{
		Id:          room.Id,
		Name:        room.Name,
		AvatarUrl:   room.AvatarUrl,
		Description: description,
		Tags:        append([]string{"全部"}, tags...),

		AvatarId:    room.AvatarId,
		Model:       room.Model,
		Vendor:      room.Vendor,
		Prompt:      room.Prompt,
		MaxContext:  room.MaxContext,
		InitMessage: room.InitMessage,
		RoomType:    room.RoomType,
		VersionMin:  room.VersionMin,
		VersionMax:  room.VersionMax,
	}
}

func (r *RoomRepo) GallerySuggests(ctx context.Context, limit int64) ([]GalleryRoom, error) {
	systemModelQ := query.Builder().Where(model2.FieldRoomGalleryRoomType, "system").
		OrderByRaw("RAND() DESC").Limit(3)
	systemModels, err := model2.NewRoomGalleryModel(r.db).Get(ctx, systemModelQ)
	if err != nil {
		return nil, err
	}

	defaultModelLimit := limit - int64(len(systemModels))
	if defaultModelLimit > 0 {
		items, err := model2.NewRoomGalleryModel(r.db).Get(
			ctx,
			query.Builder().
				Where(model2.FieldRoomGalleryRoomType, "default").
				Limit(defaultModelLimit).
				OrderByRaw("RAND()"),
		)
		if err != nil {
			return nil, err
		}

		systemModels = append(systemModels, items...)
	}

	return array.Map(systemModels, func(item model2.RoomGalleryN, _ int) GalleryRoom {
		return createGalleryRoomFromModel(item.ToRoomGallery())
	}), nil
}

func (r *RoomRepo) Galleries(ctx context.Context) ([]GalleryRoom, error) {
	items, err := model2.NewRoomGalleryModel(r.db).Get(
		ctx,
		query.Builder().
			OrderBy(model2.FieldRoomGalleryCreatedAt, "DESC"),
	)
	if err != nil {
		return nil, err
	}

	return array.Map(items, func(item model2.RoomGalleryN, _ int) GalleryRoom {
		return createGalleryRoomFromModel(item.ToRoomGallery())
	}), nil
}

func (r *RoomRepo) GalleryItem(ctx context.Context, id int64) (*GalleryRoom, error) {
	item, err := model2.NewRoomGalleryModel(r.db).First(ctx, query.Builder().Where(model2.FieldRoomGalleryId, id))
	if err != nil {
		if err == query.ErrNoResult {
			return nil, ErrNotFound
		}

		return nil, err
	}

	res := createGalleryRoomFromModel(item.ToRoomGallery())
	return &res, nil
}
