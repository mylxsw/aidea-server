package repo

import (
	"context"
	"database/sql"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/eloquent"
	"github.com/mylxsw/eloquent/query"
)

// CacheRepo 缓存仓储
type CacheRepo struct {
	db   *sql.DB
	conf *config.Config
}

// NewCacheRepo create a new CacheRepo
func NewCacheRepo(db *sql.DB, conf *config.Config) *CacheRepo {
	return &CacheRepo{db: db, conf: conf}
}

// Get 获取缓存
func (repo *CacheRepo) Get(ctx context.Context, key string) (string, error) {
	res, err := model.NewCacheModel(repo.db).First(
		ctx,
		query.Builder().
			Where(model.FieldCacheKey, key).
			Where(model.FieldCacheValidUntil, ">", time.Now()),
	)
	if err != nil && err != query.ErrNoResult {
		return "", err
	}

	if err == query.ErrNoResult {
		return "", ErrNotFound
	}

	return res.Value.ValueOrZero(), nil
}

// Set 设置缓存
// 注意：这里没有考虑并发问题，因为此处即使有并发，对业务也没有影响
func (repo *CacheRepo) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model.NewCacheModel(tx).Delete(ctx, query.Builder().Where(model.FieldCacheKey, key)); err != nil {
			return err
		}

		_, err := model.NewCacheModel(tx).Create(ctx, query.KV{
			model.FieldCacheKey:        key,
			model.FieldCacheValue:      value,
			model.FieldCacheValidUntil: time.Now().Add(ttl),
		})
		return err
	})
}

// GC 清理过期缓存
func (repo *CacheRepo) GC(ctx context.Context) error {
	_, err := model.NewCacheModel(repo.db).Delete(ctx, query.Builder().Where(model.FieldCacheValidUntil, "<", time.Now()))
	return err
}
