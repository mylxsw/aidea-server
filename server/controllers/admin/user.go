package admin

import (
	"context"
	"errors"
	"github.com/mylxsw/aidea-server/pkg/dingding"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
	"net/http"
	"strconv"
)

type UserController struct {
	repo *repo.Repository   `autowire:"@"`
	ding *dingding.Dingding `autowire:"@"`
}

func NewUserController(resolver infra.Resolver) web.Controller {
	ctl := &UserController{}
	resolver.MustAutoWire(ctl)

	return ctl
}

func (ctl *UserController) Register(router web.Router) {
	router.Group("/users", func(router web.Router) {
		router.Get("/", ctl.Users)
		router.Get("/{id}", ctl.User)
	})
}

type UserResponse struct {
	model.Users
	UserType string `json:"user_type"`
}

func NewAdminUser(user model.Users) UserResponse {
	ret := UserResponse{Users: user}
	switch int(user.UserType) {
	case repo.UserTypeInternal:
		ret.UserType = "管理员"
	case repo.UserTypeExtraPermission:
		ret.UserType = "特权用户"
	case repo.UserTypeTester:
		ret.UserType = "测试用户"
	default:
		ret.UserType = "普通用户"
	}

	return ret
}

// Users return the list of all users
// @Summary Return the list of all users
// @Tags Admin:User
// @Accept json
// @Produce json
// @Param page query integer false "Page" default(1)
// @Param per_page query integer false "Per Page" default(20)
// @Param keyword query string false "Support searching by phone number, name, and email (prefix fuzzy matching)"
// @Success 200 {object} common.Pagination[UserResponse]
// @Router /v1/admin/users [get]
func (ctl *UserController) Users(ctx context.Context, webCtx web.Context) web.Response {
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
				builder.Where(model.FieldUsersPhone, query.LIKE, keyword+"%").
					OrWhere(model.FieldUsersRealname, query.LIKE, keyword+"%").
					OrWhere(model.FieldUsersEmail, query.LIKE, keyword+"%")

				// 如果是数字，则尝试按照 ID 搜索
				ki, err := strconv.Atoi(keyword)
				if err == nil {
					builder.OrWhere(model.FieldUsersId, ki)
				}
			})
		}

		return builder.OrderBy(model.FieldUsersId, "DESC")
	}

	items, meta, err := ctl.repo.User.Users(ctx, page, perPage, opt)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(common.NewPagination(array.Map(items, func(item model.Users, _ int) UserResponse { return NewAdminUser(item) }), meta))
}

// User details
// @Summary User details
// @Tags Admin:User
// @Accept json
// @Produce json
// @Param id path integer true "User ID"
// @Success 200 {object} common.DataObj[UserResponse]
// @Router /v1/admin/users/{id} [get]
func (ctl *UserController) User(ctx context.Context, webCtx web.Context) web.Response {
	userId, err := strconv.Atoi(webCtx.PathVar("id"))
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	user, err := ctl.repo.User.GetUserByID(ctx, int64(userId))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return webCtx.JSONError(err.Error(), http.StatusNotFound)
		}
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(common.NewDataObj(NewAdminUser(*user)))
}
