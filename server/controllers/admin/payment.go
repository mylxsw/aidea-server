package admin

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"net/http"
	"strconv"
)

type PaymentController struct {
	repo *repo.Repository `autowire:"@"`
}

func NewPaymentController(resolver infra.Resolver) web.Controller {
	ctl := PaymentController{}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *PaymentController) Register(router web.Router) {
	router.Group("/payments", func(router web.Router) {
		router.Get("/histories", ctl.Histories)
	})
}

// Histories View all payment history records
func (ctl *PaymentController) Histories(ctx context.Context, webCtx web.Context) web.Response {
	page := webCtx.Int64Input("page", 1)
	if page < 1 || page > 1000 {
		page = 1
	}

	perPage := webCtx.Int64Input("per_page", 20)
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	keyword := webCtx.Input("keyword")
	opt := func(builder query.SQLBuilder) query.SQLBuilder {
		if keyword != "" {
			builder = builder.WhereGroup(func(builder query.Condition) {
				builder.Where(model.FieldPaymentHistoryPaymentId, query.LIKE, keyword+"%").
					OrWhere(model.FieldPaymentHistorySource, query.LIKE, keyword+"%")

				// 如果是数字，则尝试按照 ID 搜索
				ki, err := strconv.Atoi(keyword)
				if err == nil {
					builder.OrWhere(model.FieldPaymentHistoryUserId, ki)
				}
			})
		}

		return builder.
			Where(model.FieldPaymentHistoryStatus, repo.PaymentStatusSuccess).
			OrderBy(model.FieldPaymentHistoryId, "DESC")
	}

	items, meta, err := ctl.repo.Payment.GetPaymentHistories(ctx, page, perPage, opt)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{
		"data":      items,
		"page":      meta.Page,
		"per_page":  meta.PerPage,
		"total":     meta.Total,
		"last_page": meta.LastPage,
	})
}
