package controllers

import (
	"context"
	"errors"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"net/http"
	"strconv"
)

type ArticleController struct {
	repo *repo.Repository `autowire:"@"`
}

func NewArticleController(resolver infra.Resolver) web.Controller {
	ctl := &ArticleController{}
	resolver.MustAutoWire(ctl)
	return ctl
}

func (ctl *ArticleController) Register(router web.Router) {
	router.Group("/articles", func(router web.Router) {
		router.Get("/{id}", ctl.Article)
	})
}

// Article 文章详情查看
func (ctl *ArticleController) Article(ctx context.Context, webCtx web.Context) web.Response {
	id, err := strconv.Atoi(webCtx.PathVar("id"))
	if err != nil {
		return webCtx.JSONError("invalid id", http.StatusBadRequest)
	}

	article, err := ctl.repo.Article.Article(ctx, int64(id))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return webCtx.JSONError("article not found", http.StatusNotFound)
		}

		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"data": article})
}
