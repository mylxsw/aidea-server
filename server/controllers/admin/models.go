package admin

import (
	"context"
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
		router.Post("/", ctl.Add)
		router.Get("/{model_id}", ctl.Model)
		router.Put("/{model_id}", ctl.Update)
		router.Delete("/{model_id}", ctl.Delete)
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

// Add 添加模型
func (ctl *ModelController) Add(ctx context.Context, webCtx web.Context) web.Response {
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

// Update 更新模型
func (ctl *ModelController) Update(ctx context.Context, webCtx web.Context) web.Response {
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

// Delete 删除模型
func (ctl *ModelController) Delete(ctx context.Context, webCtx web.Context) web.Response {
	modelID := webCtx.PathVar("model_id")
	if err := ctl.repo.Model.DeleteModel(ctx, modelID); err != nil {
		return webCtx.JSONError(err.Error(), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{})
}
