package interapi

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/mylxsw/aidea-server/internal/jobs"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/redis/go-redis/v9"
)

type ManagerController struct {
	db  *sql.DB       `autowire:"@"`
	rds *redis.Client `autowire:"@"`
}

func NewManagerController(resolver infra.Resolver) web.Controller {
	ctl := &ManagerController{}
	resolver.MustAutoWire(ctl)

	return ctl
}

func (m *ManagerController) Register(router web.Router) {
	router.Group("/manager", func(router web.Router) {
		router.Post("gallery/sort", m.SortGallery)
	})
}

func (m *ManagerController) SortGallery(ctx context.Context, webCtx web.Context) web.Response {
	if err := jobs.GallerySortJob(ctx, m.db, m.rds); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}
