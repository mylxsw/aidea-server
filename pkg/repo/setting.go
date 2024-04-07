package repo

import (
	"context"
	"database/sql"
	"errors"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/eloquent"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/go-utils/array"
)

// SettingRepo 设置仓储
type SettingRepo struct {
	db   *sql.DB
	conf *config.Config
}

// NewSettingRepo create a new SettingRepo
func NewSettingRepo(db *sql.DB, conf *config.Config) *SettingRepo {
	return &SettingRepo{db: db, conf: conf}
}

// Keys 获取所有设置的 key
func (repo *SettingRepo) Keys(ctx context.Context) ([]string, error) {
	res, err := model.NewSettingModel(repo.db).Get(ctx, query.Builder().Select(model.FieldSettingKey))
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0)
	for _, r := range res {
		keys = append(keys, r.Key.ValueOrZero())
	}

	return keys, nil
}

// All 获取所有设置
func (repo *SettingRepo) All(ctx context.Context) ([]model.Setting, error) {
	res, err := model.NewSettingModel(repo.db).Get(ctx, query.Builder())
	if err != nil {
		return nil, err
	}

	settings := array.Map(res, func(s model.SettingN, _ int) model.Setting { return s.ToSetting() })
	return settings, nil
}

// Get 获取设置
func (repo *SettingRepo) Get(ctx context.Context, key string) (string, error) {
	res, err := model.NewSettingModel(repo.db).First(
		ctx,
		query.Builder().
			Where(model.FieldSettingKey, key),
	)
	if err != nil && !errors.Is(err, query.ErrNoResult) {
		return "{}", err
	}

	if errors.Is(err, query.ErrNoResult) {
		return "{}", ErrNotFound
	}

	return res.Value.ValueOrZero(), nil
}

// Set 设置设置
func (repo *SettingRepo) Set(ctx context.Context, key string, value string) error {
	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model.NewSettingModel(tx).Delete(ctx, query.Builder().Where(model.FieldSettingKey, key)); err != nil {
			return err
		}

		_, err := model.NewSettingModel(tx).Create(ctx, query.KV{
			model.FieldSettingKey:   key,
			model.FieldSettingValue: value,
		})
		return err
	})
}
