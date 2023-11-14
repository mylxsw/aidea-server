package controllers

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"net/http"
	"strings"
	"time"

	"github.com/mylxsw/glacier/infra"

	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

// ExampleController 用户提示语示例
type ExampleController struct {
	promptRepo *repo.PromptRepo `autowire:"@"`
}

// NewExampleController 创建用户提示语示例控制器
func NewExampleController(resolver infra.Resolver) web.Controller {
	ctl := ExampleController{}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *ExampleController) Register(router web.Router) {
	router.Group("/examples", func(router web.Router) {
		router.Get("/", ctl.Examples)
		router.Get("/{model}", ctl.Example)
		router.Get("/tags/{tag}", ctl.ExampleByTag)
		router.Get("/negative-prompts/{tag}", ctl.NegativePrompts)

		router.Get("/draw/prompt-tags", ctl.DrawPromptTags)
	})
}

func (ctl *ExampleController) DrawPromptTags(ctx context.Context, webCtx web.Context) web.Response {
	tags, err := ctl.promptRepo.DrawTags(ctx, repo.TagTypeCommon, repo.TagTypePrompt)
	if err != nil {
		return nil
	}

	return webCtx.JSON(web.M{
		"data": tags,
	})
}

type NegativePromptExample struct {
	Title   string `json:"title" yaml:"title"`
	Content string `json:"content,omitempty" yaml:"content,omitempty"`
}

func (ctl *ExampleController) NegativePrompts(ctx web.Context) web.Response {
	tag := ctx.PathVar("tag")
	if tag == "" {
		return ctx.JSONError("invalid tag", http.StatusBadRequest)
	}

	return ctx.JSON(web.M{
		"data": []NegativePromptExample{
			{
				Title:   "人像模式",
				Content: "EasyNegative, extra fingers, fewer fingers, floating object, makeup, face paint, mole on face, teeth, (simple background:1.3)",
			},
			{
				Title:   "人像模式增强",
				Content: "out of frame, lowres, text, error, cropped, worst quality, low quality, jpeg artifacts, ugly, duplicate, morbid, mutilated, out of frame, extra fingers, mutated hands, poorly drawn hands, poorly drawn face, mutation, deformed, blurry, dehydrated, bad anatomy, bad proportions, extra limbs, cloned face, disfigured, gross proportions, malformed limbs, missing arms, missing legs, extra arms, extra legs, fused fingers, too many fingers, long neck, username, watermark, signature",
			},
			{
				Title:   "低画质",
				Content: "easynegative, white background, (low quality, worst quality:1.4), (lowres:1.1), (long legs), greyscale, pixel art, blurry, monochrome,(text:1.8),(logo:1.8), (bad art, low detail, old), bag fingers, grainy, low quality, (mutated hands and fingers:1.5)",
			},
			{
				Title:   "通用",
				Content: "(worst quality, low quality:1.4), (bad anatomy), watermark, signature, text, logo,contact, (extra limbs),Six fingers,Low quality fingers,monochrome,(((missing arms))),(((missing legs))), (((extra arms))),(((extra legs))),less fingers,lowres, bad anatomy, bad hands, text, error, missing fingers, extra digit, fewer digits, cropped, worst quality, low quality, normal quality, jpeg artifacts, signature, watermark, username, (depth of field, bokeh, blurry:1.4),blurry background,bandages",
			},
		},
	})
}

// ExampleByTag 根据标签获取用户提示语示例
func (ctl *ExampleController) ExampleByTag(ctx web.Context) web.Response {
	tag := ctx.PathVar("tag")
	if tag == "" {
		return ctx.JSONError("invalid tag", http.StatusBadRequest)
	}

	examples, err := ctl.loadAllExamples()
	if err != nil {
		log.Errorf("Failed to load examples: %v", err)
		return ctx.JSONError(common.ErrInternalError, http.StatusInternalServerError)
	}

	return ctx.JSON(array.Filter(examples, func(example repo.PromptExample, _ int) bool {
		return array.In(tag, example.Tags)
	}))
}

// Examples 获取所有用户提示语示例
func (ctl *ExampleController) Examples(ctx web.Context) web.Response {
	examples, err := ctl.loadAllExamples()
	if err != nil {
		log.Errorf("Failed to load examples: %v", err)
		return ctx.JSONError(common.ErrInternalError, http.StatusInternalServerError)
	}

	examples = array.Map(examples, func(example repo.PromptExample, _ int) repo.PromptExample {
		if len(example.Models) == 0 && len(example.Tags) == 0 {
			example.Models = []string{"openai:gpt-"}
		}

		return example
	})

	return ctx.JSON(examples)
}

// Example 获取模型的用户提示语示例
func (ctl *ExampleController) Example(ctx web.Context) web.Response {
	model := ctx.PathVar("model")
	if model == "" {
		return ctx.JSONError(common.ErrInvalidModel, http.StatusBadRequest)
	}

	examples, err := ctl.loadAllExamples()
	if err != nil {
		log.Errorf("Failed to load examples: %v", err)
		return ctx.JSONError(common.ErrInternalError, http.StatusInternalServerError)
	}

	return ctx.JSON(array.Filter(examples, func(example repo.PromptExample, _ int) bool {
		// TODO
		return array.In(model, example.Models) || arrayContains(model, example.Models)
	}))

}

func arrayContains(text string, items []string) bool {
	for _, item := range items {
		if strings.Contains(text, item) {
			return true
		}
	}

	return false
}

func (ctl *ExampleController) loadAllExamples() ([]repo.PromptExample, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return ctl.promptRepo.CommonPromptExamples(ctx)
}
