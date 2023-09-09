package controllers

import (
	"context"
	"net/http"

	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/repo/model"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

// PromptController 提示语控制器
type PromptController struct {
	conf       *config.Config   `autowire:"@"`
	promptRepo *repo.PromptRepo `autowire:"@"`
}

// NewPromptController 创建提示语控制器
func NewPromptController(resolver infra.Resolver) web.Controller {
	var ctl PromptController
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *PromptController) Register(router web.Router) {
	router.Group("/prompts", func(router web.Router) {
		router.Get("/", ctl.Prompts)
	})
}

// Prompts 获取提示语列表
func (ctl *PromptController) Prompts(ctx context.Context, webCtx web.Context) web.Response {
	examples, err := ctl.promptRepo.ChatSystemPromptExamples(ctx)
	if err != nil {
		log.Errorf("failed to load prompts: %v", err)
		return webCtx.JSONError(common.ErrInternalError, http.StatusInternalServerError)
	}

	return webCtx.JSON(array.Map(examples, func(item model.ChatSysPromptExample, _ int) Prompt {
		return Prompt{
			Title:   item.Title,
			Content: item.Content,
		}
	}))
}

// Prompt 提示语
type Prompt struct {
	Title   string `json:"title" yaml:"title"`
	Content string `json:"content" yaml:"content"`
}
