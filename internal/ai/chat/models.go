package chat

import (
	"github.com/mylxsw/aidea-server/internal/ai/xfyun"
	"strings"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/ai/anthropic"
	"github.com/mylxsw/aidea-server/internal/ai/baichuan"
	"github.com/mylxsw/aidea-server/internal/ai/baidu"
	"github.com/mylxsw/aidea-server/internal/ai/dashscope"
	"github.com/mylxsw/aidea-server/internal/ai/gpt360"
	"github.com/mylxsw/aidea-server/internal/ai/sensenova"
	"github.com/mylxsw/aidea-server/internal/ai/tencentai"
	"github.com/mylxsw/go-utils/array"
)

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ShortName   string `json:"short_name"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Category    string `json:"category"`
	IsChat      bool   `json:"is_chat"`
	IsImage     bool   `json:"is_image"`
	Disabled    bool   `json:"disabled"`
	VersionMin  string `json:"version_min,omitempty"`
	VersionMax  string `json:"version_max,omitempty"`
	Tag         string `json:"tag,omitempty"`
}

func (m Model) RealID() string {
	segs := strings.SplitN(m.ID, ":", 2)
	return segs[1]
}

func (m Model) IsSenstiveModel() bool {
	return m.Category == "openai" || m.Category == "Anthropic"
}

func (m Model) IsVirtualModel() bool {
	return m.Category == "virtual"
}

func Models(conf *config.Config, returnAll bool) []Model {
	var models []Model
	models = append(models, openAIModels(conf)...)
	models = append(models, anthropicModels(conf)...)
	models = append(models, googleModels()...)
	models = append(models, chinaModels(conf)...)
	models = append(models, aideaModels(conf)...)

	return array.Filter(
		array.Map(models, func(item Model, _ int) Model {
			if item.ShortName == "" {
				item.ShortName = item.Name
			}

			return item
		}),
		func(item Model, _ int) bool {
			if returnAll {
				return true
			}

			return !item.Disabled
		},
	)
}

func openAIModels(conf *config.Config) []Model {
	return []Model{
		{
			ID:          "openai:gpt-3.5-turbo",
			Name:        "GPT-3.5",
			Description: "速度快，成本低",
			Category:    "openai",
			IsChat:      true,
			Disabled:    !conf.EnableOpenAI,
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/gpt35.png",
		},
		{
			ID:          "openai:gpt-3.5-turbo-16k",
			Name:        "GPT-3.5 16K",
			Description: "3.5 升级版，支持 16K 长文本",
			Category:    "openai",
			IsChat:      true,
			Disabled:    !conf.EnableOpenAI,
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/gpt35.png",
		},
		{
			ID:          "openai:gpt-4",
			Name:        "GPT-4",
			Description: "能力强，更精准",
			Category:    "openai",
			IsChat:      true,
			Disabled:    !conf.EnableOpenAI,
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/gpt4.png",
		},
		{
			ID:          "openai:gpt-4-32k",
			Name:        "GPT-4 32k",
			Description: "基于 GPT-4，但是支持4倍的内容长度",
			Category:    "openai",
			IsChat:      true,
			Disabled:    true,
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/gpt4.png",
		},
	}
}

func chinaModels(conf *config.Config) []Model {
	models := make([]Model, 0)

	models = append(models, Model{
		ID:          "讯飞星火:" + string(xfyun.ModelGeneralV1_5),
		Name:        "星火大模型V1.5",
		ShortName:   "星火 1.5",
		Description: "科大讯飞研发的认知大模型，支持语言理解、知识问答、代码编写、逻辑推理、数学解题等多元能力，服务已内嵌联网搜索功能",
		Category:    "讯飞星火",
		IsChat:      true,
		Disabled:    !conf.EnableXFYunAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/xfyun-v1.5.png",
	})

	models = append(models, Model{
		ID:          "讯飞星火:" + string(xfyun.ModelGeneralV2),
		Name:        "星火大模型V2.0",
		ShortName:   "星火 2.0",
		Description: "科大讯飞研发的认知大模型，V2.0 在 V1.5 基础上全面升级，并在代码、数学场景进行专项升级，服务已内嵌联网搜索、日期查询、天气查询、股票查询、诗词查询、字词理解等功能",
		Category:    "讯飞星火",
		IsChat:      true,
		Disabled:    !conf.EnableXFYunAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/xfyun-v2.png",
	})

	models = append(models, Model{
		ID:          "讯飞星火:" + string(xfyun.ModelGeneralV3),
		Name:        "星火大模型V3.0",
		ShortName:   "星火 3.0",
		Description: "科大讯飞研发的认知大模型，V3.0 能力全面升级，在数学、代码、医疗、教育等场景进行了专项优化，让大模型更懂你所需",
		Category:    "讯飞星火",
		IsChat:      true,
		Disabled:    !conf.EnableXFYunAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/xfyun-v3.png",
	})

	models = append(models, Model{
		ID:          "文心千帆:" + baidu.ModelErnieBotTurbo,
		Name:        "文心一言 Turbo",
		ShortName:   "文心 Turbo",
		Description: "百度研发的知识增强大语言模型，中文名是文心一言，英文名是 ERNIE Bot，能够与人对话互动，回答问题，协助创作，高效便捷地帮助人们获取信息、知识和灵感",
		Category:    "文心千帆",
		IsChat:      true,
		Disabled:    !conf.EnableBaiduWXAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/wenxinyiyan-turbo.png",
	})
	models = append(models, Model{
		ID:          "文心千帆:" + string(baidu.ModelErnieBot),
		Name:        "文心一言",
		Description: "百度研发的知识增强大语言模型增强版，中文名是文心一言，英文名是 ERNIE Bot，能够与人对话互动，回答问题，协助创作，高效便捷地帮助人们获取信息、知识和灵感",
		Category:    "文心千帆",
		IsChat:      true,
		Disabled:    !conf.EnableBaiduWXAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/creative/wenxinyiyan.png",
	})
	models = append(models, Model{
		ID:          "文心千帆:" + string(baidu.ModelErnieBot4),
		Name:        "文心一言 4.0",
		ShortName:   "文心 4.0",
		Description: "ERNIE-Bot-4 是百度自行研发的大语言模型，覆盖海量中文数据，具有更强的对话问答、内容创作生成等能力",
		Category:    "文心千帆",
		IsChat:      true,
		Disabled:    !conf.EnableBaiduWXAI,
		VersionMin:  "1.0.5",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/wenxinyiyan-4.png",
	})
	models = append(models, Model{
		ID:          "文心千帆:" + baidu.ModelLlama2_70b,
		Name:        "Llama 2 70B",
		ShortName:   "Llama2",
		Description: "由 Meta AI 研发并开源，在编码、推理及知识应用等场景表现优秀，暂不支持中文输出",
		Category:    "文心千帆",
		IsChat:      true,
		Disabled:    !conf.EnableBaiduWXAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/llama2.png",
	})
	models = append(models, Model{
		ID:          "文心千帆:" + baidu.ModelLlama2_7b_CN,
		Name:        "Llama 2 7B 中文版",
		ShortName:   "Llama2 中文",
		Description: "由 Meta AI 研发并开源，在编码、推理及知识应用等场景表现优秀，当前版本是千帆团队的中文增强版本",
		Category:    "文心千帆",
		IsChat:      true,
		Disabled:    !conf.EnableBaiduWXAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/llama2-cn.png",
	})
	models = append(models, Model{
		ID:          "文心千帆:" + baidu.ModelChatGLM2_6B_32K,
		Name:        "ChatGLM2 6B",
		ShortName:   "ChatGLM2",
		Description: "ChatGLM2-6B 是由智谱 AI 与清华 KEG 实验室发布的中英双语对话模型，具备强大的推理性能、效果、较低的部署门槛及更长的上下文",
		Category:    "文心千帆",
		IsChat:      true,
		Disabled:    !conf.EnableBaiduWXAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/chatglm.png",
	})
	models = append(models, Model{
		ID:          "文心千帆:" + baidu.ModelAquilaChat7B,
		Name:        "AquilaChat 7B",
		ShortName:   "AquilaChat",
		Description: "AquilaChat-7B 是由智源研究院研发，支持流畅的文本对话及多种语言类生成任务，通过定义可扩展的特殊指令规范",
		Category:    "文心千帆",
		IsChat:      true,
		Disabled:    !conf.EnableBaiduWXAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/aquila.png",
	})
	models = append(models, Model{
		ID:          "文心千帆:" + baidu.ModelBloomz7B,
		Name:        "BLOOMZ 7B",
		ShortName:   "BLOOMZ",
		Description: "BLOOMZ-7B 是业内知名的⼤语⾔模型，由 BigScience 研发并开源，能够以46种语⾔和13种编程语⾔输出⽂本",
		Category:    "文心千帆",
		IsChat:      true,
		Disabled:    !conf.EnableBaiduWXAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/BLOOMZ.png",
	})

	if conf.EnableDashScopeAI {
		models = append(models, Model{
			ID:          "灵积:" + dashscope.ModelQWenTurbo,
			Name:        "通义千问 Turbo",
			ShortName:   "千问 Turbo",
			Description: "通义千问超大规模语言模型，支持中文英文等不同语言输入",
			Category:    "灵积",
			IsChat:      true,
			Disabled:    !conf.EnableBaiduWXAI,
			VersionMin:  "1.0.3",
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/creative/tongyiqianwenv2.jpeg",
		})
		models = append(models, Model{
			ID:          "灵积:" + dashscope.ModelQWenPlus,
			Name:        "通义千问 Plus",
			ShortName:   "千问 Plus",
			Description: "通义千问超大规模语言模型增强版，支持中文英文等不同语言输入",
			Category:    "灵积",
			IsChat:      true,
			Disabled:    !conf.EnableBaiduWXAI,
			VersionMin:  "1.0.3",
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/creative/tongyiqianwenv2.jpeg",
		})
	}

	models = append(models, Model{
		ID:          "商汤日日新:" + string(sensenova.ModelNovaPtcXLV1),
		Name:        "商汤日日新",
		ShortName:   "日日新",
		Description: "商汤科技自主研发的超大规模语言模型，能够回答问题、创作文字，还能表达观点、撰写代码",
		Category:    "商汤日日新",
		IsChat:      true,
		Disabled:    conf.EnableSenseNovaAI,
		VersionMin:  "1.0.3",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/sensenova.png",
	})

	models = append(models, Model{
		ID:          "腾讯:" + tencentai.ModelHyllm,
		Name:        "混元大模型",
		ShortName:   "混元",
		Description: "由腾讯研发的大语言模型，具备强大的中文创作能力，复杂语境下的逻辑推理能力，以及可靠的任务执行能力",
		Category:    "腾讯",
		IsChat:      true,
		Disabled:    !conf.EnableTencentAI,
		VersionMin:  "1.0.5",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/hunyuan.png",
	})

	models = append(models, Model{
		ID:          "百川:" + baichuan.ModelBaichuan2_53B,
		Name:        "百川大模型",
		ShortName:   "百川",
		Description: "由百川智能研发的大语言模型，融合了意图理解、信息检索以及强化学习技术，结合有监督微调与人类意图对齐，在知识问答、文本创作领域表现突出",
		Category:    "百川",
		IsChat:      true,
		Disabled:    !conf.EnableBaichuan,
		VersionMin:  "1.0.5",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/baichuan.jpg",
	})

	models = append(models, Model{
		ID:          "360智脑:" + gpt360.Model360GPT_S2_V9,
		Name:        "360智脑",
		Description: "由 360 研发的大语言模型，拥有独特的语言理解能力，通过实时对话，解答疑惑、探索灵感，用AI技术帮人类打开智慧的大门",
		Category:    "360",
		IsChat:      true,
		Disabled:    !conf.EnableGPT360,
		VersionMin:  "1.0.5",
		AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/gpt360.jpg",
	})

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

func anthropicModels(conf *config.Config) []Model {
	return []Model{
		{
			ID:          "Anthropic:" + string(anthropic.ModelClaudeInstant),
			Name:        "Claude instant",
			ShortName:   "Claude",
			Description: "Anthropic's fastest model, with strength in creative tasks. Features a context window of 9k tokens (around 7,000 words).",
			Category:    "Anthropic",
			IsChat:      true,
			Disabled:    !conf.EnableAnthropic,
			VersionMin:  "1.0.5",
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/anthropic-claude-instant.png",
		},
		{
			ID:          "Anthropic:" + string(anthropic.ModelClaude2),
			Name:        "Claude 2.0",
			ShortName:   "Claude2",
			Description: "Anthropic's most powerful model. Particularly good at creative writing.",
			Category:    "Anthropic",
			IsChat:      true,
			Disabled:    !conf.EnableAnthropic,
			VersionMin:  "1.0.5",
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/anthropic-claude-2.png",
		},
	}
}

func aideaModels(conf *config.Config) []Model {
	return []Model{
		{
			ID:          "virtual:nanxian",
			Name:        "南贤大模型",
			ShortName:   "南贤",
			Description: "速度快，成本低",
			Category:    "virtual",
			IsChat:      true,
			Disabled:    !conf.EnableVirtualModel,
			VersionMin:  "1.0.5",
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/nanxian.png",
		},
		{
			ID:          "virtual:beichou",
			Name:        "北丑大模型",
			ShortName:   "北丑",
			Description: "能力强，更精准",
			Category:    "virtual",
			IsChat:      true,
			Disabled:    !conf.EnableVirtualModel,
			VersionMin:  "1.0.5",
			AvatarURL:   "https://ssl.aicode.cc/ai-server/assets/avatar/beichou.png",
		},
	}
}
