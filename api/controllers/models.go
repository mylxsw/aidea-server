package controllers

import (
	"context"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
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
		router.Get("/", ctl.Models)
		router.Get("/{category}", ctl.Model)
		router.Get("/{category}/styles", ctl.Styles)
	})
}

type ModelStyle struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Preview string `json:"preview,omitempty"`
}

func (ctl *ModelController) Styles(ctx context.Context, webCtx web.Context) web.Response {
	category := webCtx.PathVar("category")
	switch category {
	case "leapai":
		return webCtx.JSON([]ModelStyle{
			{ID: "canny", Name: "Canny", Preview: "https://www.tryleap.ai/dashboard/images/control/informationals/canny.png"},
			{ID: "mlsd", Name: "M-LSD", Preview: "https://www.tryleap.ai/dashboard/images/control/informationals/mlsd.png"},
			{ID: "pose", Name: "Pose", Preview: "https://www.tryleap.ai/dashboard/images/control/informationals/pose.png"},
			{ID: "scribble", Name: "Scribble", Preview: "https://www.tryleap.ai/dashboard/images/control/informationals/scribble.png"},
		})
	case "stabilityai":
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
	case "deepai":
		return webCtx.JSON([]ModelStyle{
			{
				ID:      "text2img",
				Name:    "文生图",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/text2imgthumb.jpeg-square_500",
			},
			{
				ID:      "cute-creature-generator",
				Name:    "可爱的动物",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/panda.jpeg-square_500",
			},
			{
				ID:      "fantasy-world-generator",
				Name:    "奇幻世界",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/sf_fantasy.jpeg-square_500",
			},
			{
				ID:      "cyberpunk-generator",
				Name:    "未来科幻",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/cyborg.jpeg-square_500",
			},
			{
				ID:      "anime-portrait-generator",
				Name:    "动漫人物",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/anime_demo.jpeg-square_500",
			},
			{
				ID:      "old-style-generator",
				Name:    "老式风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/old_skool.jpeg-square_500",
			},
			{
				ID:      "renaissance-painting-generator",
				Name:    "文艺复兴风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/renais.jpeg-square_500",
			},
			{
				ID:      "abstract-painting-generator",
				Name:    "抽象风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/abstracttt.jpeg-square_500",
			},
			{
				ID:      "impressionism-painting-generator",
				Name:    "印象派风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/impr.jpeg-square_500",
			},
			{
				ID:      "surreal-graphics-generator",
				Name:    "超现实风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/cupcoff.jpeg-square_500",
			},
			{
				ID:      "3d-objects-generator",
				Name:    "3D物体",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/cmpre.jpeg-square_500",
			},
			{
				ID:      "origami-3d-generator",
				Name:    "折纸风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/origami.jpeg-square_500",
			},
			{
				ID:      "hologram-3d-generator",
				Name:    "全息",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/hologram.jpeg-square_500",
			},
			{
				ID:      "3d-character-generator",
				Name:    "3D人物",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/tinkerbell.jpeg-square_500",
			},
			{
				ID:      "watercolor-painting-generator",
				Name:    "水彩风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/watercolour.jpeg-square_500",
			},
			{
				ID:      "pop-art-generator",
				Name:    "流行艺术风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/popart.jpeg-square_500",
			},
			{
				ID:      "contemporary-architecture-generator",
				Name:    "现代建筑",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/contemporary.jpeg-square_500",
			},
			{
				ID:      "future-architecture-generator",
				Name:    "未来建筑",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/future.jpeg-square_500",
			},
			{
				ID:      "watercolor-architecture-generator",
				Name:    "水彩建筑",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/water_arch.jpeg-square_500",
			},
			{
				ID:      "fantasy-character-generator",
				Name:    "奇幻角色",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/fantasy_rabbit.jpeg-square_500",
			},
			{
				ID:      "steampunk-generator",
				Name:    "蒸汽朋克风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/steampunk.jpeg-square_500",
			},
			{
				ID:      "logo-generator",
				Name:    "Logo",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/logos.jpeg-square_500",
			},
			{
				ID:      "pixel-art-generator",
				Name:    "像素风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/pixel.jpeg-square_500",
			},
			{
				ID:      "street-art-generator",
				Name:    "街头艺术风格",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/streetart.jpeg-square_500",
			},
			{
				ID:      "surreal-portrait-generator",
				Name:    "超现实人物",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/goddess.jpg",
			},
			{
				ID:      "anime-world-generator",
				Name:    "动漫世界",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/animecat.jpg",
			},
			{
				ID:      "fantasy-portrait-generator",
				Name:    "奇幻肖像",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/fantasydoll.jpeg-square_500",
			},
			{
				ID:      "comics-portrait-generator",
				Name:    "漫画肖像",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/comicsman.jpeg-square_500",
			},
			{
				ID:      "cyberpunk-portrait-generator",
				Name:    "赛博朋克肖像",
				Preview: "https://ssl.aicode.cc/ai-server/assets/deepai/mage.jpeg-square_500",
			},
		})
	}

	return webCtx.JSON([]ModelStyle{})
}

// Model 模型信息
func (ctl *ModelController) Model(ctx web.Context, client *auth.ClientInfo) web.Response {
	category := ctx.PathVar("category")

	switch category {
	case "openai":
		return ctx.JSON(openAIModels(ctl.conf))
	case "deepai":
		return ctx.JSON(deepAIModels())
	case "stabilityai":
		return ctx.JSON(stabilityAIModels())
	}

	return ctx.JSON(web.M{})
}

// Models 获取模型列表
func (ctl *ModelController) Models(ctx web.Context, client *auth.ClientInfo) web.Response {
	models := []Model{}
	models = append(models, openAIModels(ctl.conf)...)
	models = append(models, claudeModels()...)
	models = append(models, googleModels()...)
	models = append(models, chinaModels(ctl.conf)...)

	models = array.Filter(models, func(item Model, _ int) bool {
		//if item.Disabled && client.Platform == "ios" {
		//	return false
		//}
		if item.Disabled {
			return false
		}

		if item.VersionMin != "" && helper.VersionOlder(client.Version, item.VersionMin) {
			return false
		}

		if item.VersionMax != "" && helper.VersionNewer(client.Version, item.VersionMax) {
			return false
		}

		return true
	})

	return ctx.JSON(models)
}

func stabilityAIModels() []Model {
	return []Model{
		{
			ID:          "stabilityai:stable-diffusion-v1",
			Name:        "stable-diffusion-v1",
			Description: "Stability-AI Stable Diffusion v1.4",
			Category:    "stabilityai",
			IsImage:     true,
		},
		{
			ID:          "stabilityai:stable-diffusion-v1-5",
			Name:        "stable-diffusion-v1-5",
			Description: "Stability-AI Stable Diffusion v1.5",
			Category:    "stabilityai",
			IsImage:     true,
		},
		{
			ID:          "stabilityai:stable-diffusion-512-v2-0",
			Name:        "stable-diffusion-512-v2-0",
			Description: "Stability-AI Stable Diffusion v2.0",
			Category:    "stabilityai",
			IsImage:     true,
		},
		{
			ID:          "stabilityai:stable-diffusion-768-v2-0",
			Name:        "stable-diffusion-768-v2-0",
			Description: "Stability-AI Stable Diffusion 768 v2.0",
			Category:    "stabilityai",
			IsImage:     true,
		},
		{
			ID:          "stabilityai:stable-diffusion-512-v2-1",
			Name:        "stable-diffusion-512-v2-1",
			Description: "Stability-AI Stable Diffusion v2.1",
			Category:    "stabilityai",
			IsImage:     true,
		},
		{
			ID:          "stabilityai:stable-diffusion-768-v2-1",
			Name:        "stable-diffusion-768-v2-1",
			Description: "Stability-AI Stable Diffusion 768 v2.1",
			Category:    "stabilityai",
			IsImage:     true,
		},
		{
			ID:          "stabilityai:stable-diffusion-xl-beta-v2-2-2",
			Name:        "stable-diffusion-xl-beta-v2-2-2",
			Description: "Stability-AI Stable Diffusion XL Beta v2.2.2",
			Category:    "stabilityai",
			IsImage:     true,
		},
	}
}

func deepAIModels() []Model {
	return []Model{
		{
			ID:          "deepai:text2img",
			Name:        "text2img",
			Description: "根据文本描述创建图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:cute-creature-generator",
			Name:        "cute-creature-generator",
			Description: "生成可爱的动物图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:fantasy-world-generator",
			Name:        "fantasy-world-generator",
			Description: "生成奇幻世界图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:cyberpunk-generator",
			Name:        "cyberpunk-generator",
			Description: "生成未来科幻图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:anime-portrait-generator",
			Name:        "anime-portrait-generator",
			Description: "生成动漫人物图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:old-style-generator",
			Name:        "old-style-generator",
			Description: "生成老式风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:renaissance-painting-generator",
			Name:        "renaissance-painting-generator",
			Description: "生成文艺复兴风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:abstract-painting-generator",
			Name:        "abstract-painting-generator",
			Description: "生成抽象风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:impressionism-painting-generator",
			Name:        "impressionism-painting-generator",
			Description: "生成印象派风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:surreal-graphics-generator",
			Name:        "surreal-graphics-generator",
			Description: "生成超现实风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:3d-objects-generator",
			Name:        "3d-objects-generator",
			Description: "生成3D物体图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:origami-3d-generator",
			Name:        "origami-3d-generator",
			Description: "生成折纸风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:hologram-3d-generator",
			Name:        "hologram-3d-generator",
			Description: "生成全息图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:3d-character-generator",
			Name:        "3d-character-generator",
			Description: "生成3D人物图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:watercolor-painting-generator",
			Name:        "watercolor-painting-generator",
			Description: "生成水彩风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:pop-art-generator",
			Name:        "pop-art-generator",
			Description: "生成流行艺术风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:contemporary-architecture-generator",
			Name:        "contemporary-architecture-generator",
			Description: "生成现代建筑图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:future-architecture-generator",
			Name:        "future-architecture-generator",
			Description: "生成未来建筑图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:watercolor-architecture-generator",
			Name:        "watercolor-architecture-generator",
			Description: "生成水彩建筑图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:fantasy-character-generator",
			Name:        "fantasy-character-generator",
			Description: "生成奇幻人物图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:steampunk-generator",
			Name:        "steampunk-generator",
			Description: "生成蒸汽朋克风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:logo-generator",
			Name:        "logo-generator",
			Description: "生成Logo图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:pixel-art-generator",
			Name:        "pixel-art-generator",
			Description: "生成像素风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:street-art-generator",
			Name:        "street-art-generator",
			Description: "生成街头艺术风格图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:surreal-portrait-generator",
			Name:        "surreal-portrait-generator",
			Description: "生成超现实人物图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:anime-world-generator",
			Name:        "anime-world-generator",
			Description: "生成动漫世界图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:fantasy-portrait-generator",
			Name:        "fantasy-portrait-generator",
			Description: "生成奇幻人物图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:comics-portrait-generator",
			Name:        "comics-portrait-generator",
			Description: "生成漫画人物图像",
			Category:    "deepai",
			IsImage:     true,
		},
		{
			ID:          "deepai:cyberpunk-portrait-generator",
			Name:        "cyberpunk-portrait-generator",
			Description: "生成未来科幻人物图像",
			Category:    "deepai",
			IsImage:     true,
		},
	}
}

func chinaModels(conf *config.Config) []Model {
	models := make([]Model, 0)

	if conf.EnableXFYunAI {
		models = append(models, Model{
			ID:          "讯飞星火:generalv2",
			Name:        "星火大模型V2.0",
			Description: "科大讯飞研发的认知大模型，拥有跨领域的知识和语言理解能力，完成问答对话和文学创作等任务",
			Category:    "讯飞星火",
			IsChat:      true,
			Disabled:    false,
			VersionMin:  "1.0.3",
		})
	}

	if conf.EnableBaiduWXAI {
		models = append(models, Model{
			ID:          "文心千帆:model_ernie_bot_turbo",
			Name:        "文心一言",
			Description: "百度研发的知识增强大语言模型，中文名是文心一言，英文名是ERNIE Bot，能够与人对话互动，回答问题，协助创作，高效便捷地帮助人们获取信息、知识和灵感",
			Category:    "文心千帆",
			IsChat:      true,
			Disabled:    false,
			VersionMin:  "1.0.3",
		})
	}

	if conf.EnableDashScopeAI {
		models = append(models, Model{
			ID:          "灵积:qwen-v1",
			Name:        "通义千问",
			Description: "阿里达摩院自主研发的超大规模语言模型，能够回答问题、创作文字，还能表达观点、撰写代码",
			Category:    "灵积",
			IsChat:      true,
			Disabled:    false,
			VersionMin:  "1.0.3",
		})
	}

	return models
}

func googleModels() []Model {
	return []Model{
		{
			ID:          "google:bard",
			Name:        "Bard",
			Description: "As a creative and helpful collaborator, Bard can supercharge your imagination, boost your productivity, and help you bring your ideas to life-whether you want help planning the perfect birthday party and drafting the invitation, creating a pro & con list for a big decision, or understanding really complex topics simply.",
			Category:    "google",
			IsChat:      true,
			Disabled:    true,
		},
	}
}

func claudeModels() []Model {
	return []Model{
		{
			ID:          "claude:claude-instant",
			Name:        "Claude-instant",
			Description: "Anthropic's fastest model, with strength in creative tasks. Features a context window of 9k tokens (around 7,000 words).",
			Category:    "claude",
			IsChat:      true,
			Disabled:    true,
		},
		{
			ID:          "claude:claude+",
			Name:        "Claude+",
			Description: "Anthropic's most powerful model. Particularly good at creative writing.",
			Category:    "claude",
			IsChat:      true,
			Disabled:    true,
		},
	}
}

func openAIModels(conf *config.Config) []Model {
	if !conf.EnableOpenAI {
		return []Model{}
	}

	return []Model{
		{
			ID:          "openai:gpt-3.5-turbo",
			Name:        "GPT-3.5",
			Description: "速度快，成本低",
			Category:    "openai",
			IsChat:      true,
		},
		{
			ID:          "openai:gpt-3.5-turbo-16k",
			Name:        "GPT-3.5 16K",
			Description: "3.5 升级版，支持 16K 长文本",
			Category:    "openai",
			IsChat:      true,
		},
		{
			ID:          "openai:gpt-4",
			Name:        "GPT-4",
			Description: "能力强，更精准",
			Category:    "openai",
			IsChat:      true,
			Disabled:    false,
			// VersionMin:  "1.0.2",
			// Tag:         "实验性功能",
		},
		{
			ID:          "openai:gpt-4-32k",
			Name:        "GPT-4 32k",
			Description: "基于 GPT-4，但是支持4倍的内容长度",
			Category:    "openai",
			IsChat:      true,
			Disabled:    true,
		},
		// {
		// 	ID:          "openai:DALL·E",
		// 	Name:        "DALL·E",
		// 	Description: "根据自然语言创建现实的图像和艺术",
		// 	IsImage:     true,
		// },
	}
}

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsChat      bool   `json:"is_chat"`
	IsImage     bool   `json:"is_image"`
	Disabled    bool   `json:"disabled"`
	VersionMin  string `json:"version_min,omitempty"`
	VersionMax  string `json:"version_max,omitempty"`
	Tag         string `json:"tag,omitempty"`
}
