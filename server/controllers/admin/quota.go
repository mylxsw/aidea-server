package admin

import (
	"context"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/dingding"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"net/http"
	"strconv"
	"time"
)

type QuotaController struct {
	repo *repo.Repository   `autowire:"@"`
	ding *dingding.Dingding `autowire:"@"`
}

func NewQuotaController(resolver infra.Resolver) web.Controller {
	ctl := &QuotaController{}
	resolver.MustAutoWire(ctl)

	return ctl
}

func (ctl *QuotaController) Register(router web.Router) {
	router.Group("/quotas", func(router web.Router) {
		router.Post("/assign", ctl.AssignQuotaToUser)
		router.Get("/users/{id}", ctl.UserQuotas)
	})
}

type AssignQuotaReq struct {
	// UserID 用户 ID
	UserID int64 `json:"user_id"`
	// Quota 分配智慧果数量
	Quota int64 `json:"quota"`
	// ValidPeriod 有效期，单位小时
	ValidPeriod int64 `json:"valid_period,omitempty"`
	// Note 备注
	Note string `json:"note,omitempty"`
}

// AssignQuotaToUser Assign smart fruits to users
// @Summary Assign smart fruits to users
// @Tags Admin:Quotas
// @Accept json
// @Produce json
// @Param request body AssignQuotaReq true "AssignQuotaReq"
// @Success 200 {object} common.EmptyResponse
// @Router /v1/admin/quotas/assign [post]
func (ctl *QuotaController) AssignQuotaToUser(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	var req AssignQuotaReq
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError("invalid request", http.StatusBadRequest)
	}

	if req.UserID <= 0 {
		return webCtx.JSONError("invalid user id", http.StatusBadRequest)
	}

	if req.Quota <= 0 || req.Quota > 100000000 {
		return webCtx.JSONError("invalid quota", http.StatusBadRequest)
	}

	if req.Note == "" {
		req.Note = "Administrator assignment"
	}

	if req.ValidPeriod <= 0 {
		req.ValidPeriod = 24 * 365 * 10
	}
	expireTime := time.Now().Add(time.Duration(req.ValidPeriod) * time.Hour)

	targetUser, err := ctl.repo.User.GetUserByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return webCtx.JSONError("user not found", http.StatusBadRequest)
		}

		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	if _, err := ctl.repo.Quota.AddUserQuota(ctx, req.UserID, req.Quota, expireTime, req.Note, ""); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	message := fmt.Sprintf("管理员 %d 为用户 %d 分配 %d 个智慧果", user.ID, targetUser.Id, req.Quota)
	if err := ctl.ding.Send(dingding.NewMarkdownMessage(message, message, []string{})); err != nil {
		log.Errorf("send dingding message failed: %s", err.Error())
	}

	return webCtx.JSON(web.M{})
}

type UserQuotaResponse struct {
	// Details Smart Fruit User Details
	Details []repo.Quota `json:"details"`
	// Total remaining Smart Fruit
	Total int64 `json:"total"`
}

// UserQuotas User Smart Fruit Details
// @Summary User Smart Fruit Details
// @Tags Admin:Quotas
// @Produce json
// @Param id path integer true "User ID"
// @Success 200 {object} UserQuotaResponse
// @Router /v1/admin/quotas/users/{id} [get]
func (ctl *QuotaController) UserQuotas(ctx context.Context, webCtx web.Context) web.Response {
	userId, err := strconv.Atoi(webCtx.PathVar("id"))
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	quotas, err := ctl.repo.Quota.GetUserQuotaDetails(ctx, int64(userId))
	if err != nil {
		log.Errorf("get user quota failed: %s", err)
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	var rest int64
	for _, quota := range quotas {
		if quota.Expired || quota.Rest <= 0 {
			continue
		}

		rest += quota.Rest
	}

	return webCtx.JSON(UserQuotaResponse{
		Details: quotas,
		Total:   rest,
	})
}
