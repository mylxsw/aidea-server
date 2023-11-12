package v2

import (
	"context"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/web"
)

// ModelController 模型控制器
type ModelController struct {
	conf *config.Config
}

// NewModelController 创建模型控制器
func NewModelController(conf *config.Config) web.Controller {
	return &ModelController{conf: conf}
}

func (ctl *ModelController) Register(router web.Router) {
	router.Group("/models", func(router web.Router) {
		router.Get("/styles", ctl.Styles)
	})
}

type ModelStyle struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Preview string `json:"preview,omitempty"`
}

func (ctl *ModelController) Styles(ctx context.Context, webCtx web.Context) web.Response {
	return webCtx.JSON([]ModelStyle{
		{ID: "enhance", Name: "效果增强", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/enhance.png-square_500"},
		{ID: "anime", Name: "日本动漫", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/anime.png-square_500"},
		{ID: "photographic", Name: "摄影", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/photographic.png-square_500"},
		{ID: "digital-art", Name: "数字艺术", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/digital-art.png-square_500"},
		{ID: "comic-book", Name: "漫画书", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/comic-book.png-square_500"},
		{ID: "fantasy-art", Name: "奇幻艺术", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/fantasy-art.png-square_500"},
		{ID: "analog-film", Name: "模拟电影", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/analog-film.png-square_500"},
		{ID: "neon-punk", Name: "赛博朋克", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/neon-punk.png-square_500"},
		{ID: "isometric", Name: "等距视角", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/isometric.png-square_500"},
		{ID: "low-poly", Name: "低多边形", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/low-poly.png-square_500"},
		{ID: "origami", Name: "折纸", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/origami.png-square_500"},
		{ID: "line-art", Name: "线条艺术", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/line-art.png-square_500"},
		{ID: "modeling-compound", Name: "粘土工艺", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/modeling-compound.png-square_500"},
		{ID: "cinematic", Name: "电影风格", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/cinematic.png-square_500"},
		{ID: "3d-model", Name: "3D 模型", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/3d-model.png-square_500"},
		{ID: "pixel-art", Name: "像素艺术", Preview: "https://ssl.aicode.cc/ai-server/assets/stability.ai/pixel-art.png-square_500"},
	})
}
