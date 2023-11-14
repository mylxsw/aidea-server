package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"time"

	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/array"
	"github.com/redis/go-redis/v9"
)

type GalleryService struct {
	creativeRepo *repo.CreativeRepo `autowire:"@"`
	rds          *redis.Client      `autowire:"@"`
}

func NewGalleryService(resolver infra.Resolver) *GalleryService {
	srv := &GalleryService{}
	resolver.MustAutoWire(srv)

	return srv
}

type GalleryPaginateResponse struct {
	query.PaginateMeta
	Data []model.CreativeGallery `json:"data"`
}

func (srv *GalleryService) Gallery(ctx context.Context, page, perPage int64) (*GalleryPaginateResponse, error) {
	// 注意：在 jobs.GallerySortJob 中会更新这个缓存
	key := fmt.Sprintf("gallery-list:%d:%d", page, perPage)
	if res, err := srv.rds.Get(ctx, key).Result(); err == nil {
		var resp GalleryPaginateResponse
		if err := json.Unmarshal([]byte(res), &resp); err == nil {
			return &resp, nil
		}
	}

	items, meta, err := srv.creativeRepo.Gallery(ctx, page, perPage)
	if err != nil {
		return nil, err
	}

	items = array.Map(items, func(item model.CreativeGallery, _ int) model.CreativeGallery {
		item.Prompt = misc.SubString(item.Prompt, 100)
		return item
	})

	resp := GalleryPaginateResponse{
		PaginateMeta: meta,
		Data:         items,
	}

	if data, err := json.Marshal(resp); err != nil {
		return nil, err
	} else {
		if err := srv.rds.Set(ctx, key, data, 6*time.Hour).Err(); err != nil {
			return nil, err
		}
	}

	return &resp, nil
}
