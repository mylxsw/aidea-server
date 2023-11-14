package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/rate"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/must"
	"github.com/redis/go-redis/v9"
	"time"
)

type ChatService struct {
	rds     *redis.Client     `autowire:"@"`
	limiter *rate.RateLimiter `autowire:"@"`
	rep     *repo.Repository  `autowire:"@"`
}

func NewChatService(resolver infra.Resolver) *ChatService {
	svc := &ChatService{}
	resolver.MustAutoWire(svc)
	return svc
}

func (svc *ChatService) Room(ctx context.Context, userID int64, roomID int64) (*model.Rooms, error) {
	roomKey := fmt.Sprintf("chat-room:%d:%d:info", userID, roomID)
	if roomStr, err := svc.rds.Get(ctx, roomKey).Result(); err == nil {
		var room model.Rooms
		if err := json.Unmarshal([]byte(roomStr), &room); err == nil {
			return &room, nil
		}
	}

	room, err := svc.rep.Room.Room(ctx, userID, roomID)
	if err != nil {
		return nil, err
	}

	if err := svc.rds.SetNX(ctx, roomKey, string(must.Must(json.Marshal(room))), 60*time.Minute).Err(); err != nil {
		return nil, err
	}

	return room, nil
}
