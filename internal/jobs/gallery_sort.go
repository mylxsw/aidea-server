package jobs

import (
	"context"
	"database/sql"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"math/rand"
	"time"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/eloquent/query"
	"github.com/redis/go-redis/v9"
	"gopkg.in/guregu/null.v3"
)

func GallerySortJob(ctx context.Context, db *sql.DB, rds *redis.Client) error {
	// 随机增加热度
	randomUpdateGalleriesHotValue(ctx, db)

	// 查询最新的排序
	q := query.Builder().
		Select(model.FieldCreativeGalleryId).
		Where(model.FieldCreativeGalleryStatus, repo.CreativeGalleryStatusOK).
		OrderByRaw("RAND()*hot_value+id DESC")

	galleries, err := model.NewCreativeGalleryModel(db).Get(ctx, q)
	if err != nil {
		log.Errorf("get galleries failed for GallerySortJob: %v", err)
		return err
	}

	// 更新 Gallery 随机排序数据库
	if err := query.Transaction(db, func(tx query.Database) error {
		if _, err := model.NewCreativeGalleryRandomModel(tx).Delete(ctx, query.Builder()); err != nil {
			log.Errorf("delete random gallery failed for GallerySortJob: %v", err)
			return err
		}

		for i, gallery := range galleries {
			if _, err := model.NewCreativeGalleryRandomModel(tx).Create(ctx, query.KV{
				model.FieldCreativeGalleryRandomId:        i + 1,
				model.FieldCreativeGalleryRandomGalleryId: gallery.Id,
			}); err != nil {
				log.Errorf("create random gallery failed for GallerySortJob: %v", err)
				return err
			}
		}

		return nil
	}); err != nil {
		return err
	}

	// 清空缓存
	keys, _ := rds.Keys(ctx, "gallery-list:*").Result()
	for _, key := range keys {
		if err := rds.Del(ctx, key).Err(); err != nil {
			log.Errorf("delete redis key [%s] failed: %v", key, err)
		}
	}

	return nil
}

func randomUpdateGalleriesHotValue(ctx context.Context, db *sql.DB) {
	galleries, err := model.NewCreativeGalleryModel(db).Get(ctx, query.Builder().Where(model.FieldCreativeGalleryStatus, repo.CreativeGalleryStatusOK))
	if err != nil {
		log.Errorf("get galleries failed for GallerySortJob: %v", err)
		return
	}

	for _, gallery := range galleries {
		var delta int64
		if gallery.CreatedAt.ValueOrZero().After(time.Now().Add(-24 * time.Hour)) {
			delta = gallery.Id.ValueOrZero()
		}

		gallery.HotValue = null.IntFrom(gallery.HotValue.ValueOrZero() + rand.Int63n(100+delta))
		if err := gallery.Save(ctx, model.FieldCreativeGalleryHotValue); err != nil {
			log.Errorf("random update gallery hot value failed: %v", err)
		}
	}
}
