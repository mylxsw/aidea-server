package repo

import (
	"context"
	"database/sql"
	"errors"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/eloquent/query"
)

type ArticleRepo struct {
	db   *sql.DB
	conf *config.Config
}

// NewArticleRepo create a new ArticleRepo
func NewArticleRepo(db *sql.DB, conf *config.Config) *ArticleRepo {
	return &ArticleRepo{db: db, conf: conf}
}

func (repo *ArticleRepo) Article(ctx context.Context, id int64) (*model.Articles, error) {
	item, err := model.NewArticlesModel(repo.db).First(ctx, query.Builder().Where(model.FieldArticlesId, id))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	ret := item.ToArticles()
	return &ret, nil
}
