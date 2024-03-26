package admin

import (
	"context"
	"errors"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/service"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
	"net/http"
)

type ModelController struct {
	repo *repo.Repository `autowire:"@"`
	svc  *service.Service `autowire:"@"`
}

func NewModelController(resolver infra.Resolver) web.Controller {
	ctl := &ModelController{}
	resolver.MustAutoWire(ctl)

	return ctl
}

func (ctl *ModelController) Register(router web.Router) {
	router.Group("/models", func(router web.Router) {
		router.Get("/", ctl.Models)
		router.Post("/", ctl.CreateModel)
		router.Get("/{model_id}", ctl.Model)
		router.Put("/{model_id}", ctl.UpdateModel)
		router.Delete("/{model_id}", ctl.DeleteModel)
	})

	router.Group("/free-models/daily", func(router web.Router) {
		router.Get("/", ctl.DailyFreeModels)
		router.Get("/{model_id}", ctl.DailyFreeModel)
		router.Post("/{model_id}", ctl.AddDailyFreeModel)
		router.Put("/{model_id}", ctl.UpdateDailyFreeModel)
		router.Delete("/{model_id}", ctl.DeleteDailyFreeModel)
	})
}

// Models 返回所有的模型列表
// - sort: 排序字段，支持 id:desc, 默认为空
func (ctl *ModelController) Models(ctx context.Context, webCtx web.Context) web.Response {
	sort := webCtx.Input("sort")
	if !array.In(sort, []string{"", "id:desc"}) {
		return webCtx.JSONError("invalid sort parameter", http.StatusBadRequest)
	}

	opt := func(q query.SQLBuilder) query.SQLBuilder {
		if sort == "id:desc" {
			q = q.OrderBy("id", "desc")
		}

		return q
	}

	models, err := ctl.repo.Model.GetModels(ctx, opt)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"data": models})
}

// Model 返回指定模型的详细信息
func (ctl *ModelController) Model(ctx context.Context, webCtx web.Context) web.Response {
	modelID := webCtx.PathVar("model_id")
	model, err := ctl.repo.Model.GetModel(ctx, modelID)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"data": model})
}

// CreateModel 添加模型
func (ctl *ModelController) CreateModel(ctx context.Context, webCtx web.Context) web.Response {
	var req repo.ModelAddReq
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	if req.ModelID == "" {
		return webCtx.JSONError("模型ID不能为空", http.StatusBadRequest)
	}

	if req.Name == "" {
		return webCtx.JSONError("模型名称不能为空", http.StatusBadRequest)
	}

	req.Status = repo.ModelStatusEnabled

	id, err := ctl.repo.Model.AddModel(ctx, req)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"id": id})
}

// UpdateModel 更新模型
func (ctl *ModelController) UpdateModel(ctx context.Context, webCtx web.Context) web.Response {
	modelID := webCtx.PathVar("model_id")
	var req repo.ModelUpdateReq
	if err := webCtx.Unmarshal(&req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	if req.Name == "" {
		return webCtx.JSONError("模型名称不能为空", http.StatusBadRequest)
	}

	if err := ctl.repo.Model.UpdateModel(ctx, modelID, req); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// DeleteModel 删除模型
func (ctl *ModelController) DeleteModel(ctx context.Context, webCtx web.Context) web.Response {
	modelID := webCtx.PathVar("model_id")
	if err := ctl.repo.Model.DeleteModel(ctx, modelID); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// DailyFreeModels 返回所有的免费模型列表
func (ctl *ModelController) DailyFreeModels(ctx context.Context, webCtx web.Context) web.Response {
	models, err := ctl.repo.Model.DailyFreeModels(ctx)
	if err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"data": models})
}

// DailyFreeModel 返回指定的免费模型信息
func (ctl *ModelController) DailyFreeModel(ctx context.Context, webCtx web.Context) web.Response {
	modelID := webCtx.PathVar("model_id")
	model, err := ctl.repo.Model.GetDailyFreeModel(ctx, modelID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return webCtx.JSONError("model not found", http.StatusNotFound)
		}
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"data": model})
}

// AddDailyFreeModel 添加免费模型
func (ctl *ModelController) AddDailyFreeModel(ctx context.Context, webCtx web.Context) web.Response {
	var mod coins.ModelWithName
	if err := webCtx.Unmarshal(&mod); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	if mod.Model == "" {
		return webCtx.JSONError("模型 ID 不能为空", http.StatusBadRequest)
	}

	if mod.Name == "" {
		return webCtx.JSONError("模型名称不能为空", http.StatusBadRequest)
	}

	if mod.FreeCount == 0 {
		return webCtx.JSONError("免费次数不能为空", http.StatusBadRequest)
	}

	if err := ctl.repo.Model.AddDailyFreeModel(ctx, mod); err != nil {
		if errors.Is(err, repo.ErrAlreadyExists) {
			return webCtx.JSONError("model already exists", http.StatusConflict)
		}

		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// UpdateDailyFreeModel 更新免费模型
func (ctl *ModelController) UpdateDailyFreeModel(ctx context.Context, webCtx web.Context) web.Response {
	modelID := webCtx.PathVar("model_id")
	var mod coins.ModelWithName
	if err := webCtx.Unmarshal(&mod); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusBadRequest)
	}

	if mod.Name == "" {
		return webCtx.JSONError("模型名称不能为空", http.StatusBadRequest)
	}

	if mod.FreeCount == 0 {
		return webCtx.JSONError("免费次数不能为空", http.StatusBadRequest)
	}

	if err := ctl.repo.Model.UpdateDailyFreeModel(ctx, modelID, mod); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}

// DeleteDailyFreeModel 删除免费模型
func (ctl *ModelController) DeleteDailyFreeModel(ctx context.Context, webCtx web.Context) web.Response {
	modelID := webCtx.PathVar("model_id")
	if err := ctl.repo.Model.DeleteDailyFreeModel(ctx, modelID); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}
