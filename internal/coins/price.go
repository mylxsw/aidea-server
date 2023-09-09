package coins

import (
	"fmt"
	"math"
	"time"
)

type CoinTable map[string]int64

var coinTables = map[string]CoinTable{
	"openai": {
		// 1000 Token 计费
		"gpt-3.5-turbo":          5,  // valid
		"gpt-3.5-turbo-0613":     5,  // valid
		"gpt-3.5-turbo-16k":      10, // valid
		"gpt-3.5-turbo-16k-0613": 10, // valid
		"gpt-4":                  50, // valid
		"gpt-4-8k":               50, // valid
		"gpt-4-32k":              200,
		// 每次计费
		"DALL·E": 50,

		// 国产模型
		"model_ernie_bot_turbo": 5, // valid 文心一言
		"qwen-v1":               5, // valid 通义千问
		"generalv2":             5, // valid 讯飞星火 v2
		"general":               3, // valid 讯飞星火 v1.5
	},
	"deepai": {
		"default": 30, // valid
	},
	"leapai": {
		"default": 30, // valid
	},
	"fromston": {
		"default": 30, // valid
	},
	"stabilityai": {
		"default": 300, // 默认值，当找不到匹配时使用，一般不会用到

		// 目前使用的是 30 步
		"image-step30-512x512":   20, // valid
		"image-step30-768x768":   30, // valid
		"image-step30-1024x1024": 50, // valid

		"image-step50-512x512":   20,
		"image-step50-768x768":   40,
		"image-step50-1024x1024": 60,

		"image-step100-512x512":   40,
		"image-step100-768x768":   80,
		"image-step100-1024x1024": 160,
		"image-step150-512x512":   60,
		"image-step150-768x768":   120,
		"image-step150-1024x1024": 240,

		"stable-diffusion-xl-1024-v0-9-30":  30,
		"stable-diffusion-xl-1024-v0-9-50":  60,
		"stable-diffusion-xl-1024-v0-9-100": 100,

		"upscale-esrgan-v1-x2plus":                    20,
		"upscale-stable-diffusion-x4-latent-upscaler": 300,
	},
	"voice-recognition": {
		"tencent": 1, // valid
	},
	"translate": {
		"youdao": 0,
	},
	"upload": {
		"qiniu": 1,
	},
}

// 价格表
type PriceTable map[string]float64

var priceTables = map[string]PriceTable{
	"openai": {
		"gpt-3.5-turbo":          0.014,
		"gpt-3.5-turbo-0613":     0.014,
		"gpt-3.5-turbo-16k":      0.028,
		"gpt-3.5-turbo-16k-0613": 0.028,
		"gpt-4":                  0.546, // 原价 0.42，因为使用了三方 API，价格上浮 30%
		"gpt-4-8k":               0.546, // 原价 0.42，因为使用了三方 API，价格上浮 30%
		"gpt-4-32k":              1.092, // 原价 0.84，因为使用了三方 API，价格上浮 30%
		"DALL·E":                 0.14,
	},
	"deepai": {
		"default": 0.07, // 图片生成固定成本
	},
	"leapai": {
		"default": 0.035, // 图片生成固定成本
	},
	"fromston": {
		"default": 0.525, // 图片生成固定成本(实际上受 分辨率，禅思模式，三方模型影响)
	},
	"stabilityai": {
		"default":                0.5, // 默认值，当找不到匹配时使用，一般不会用到
		"image-step30-512x512":   0.014,
		"image-step30-512x768":   0.035,
		"image-step30-512x1024":  0.056,
		"image-step30-768x768":   0.07,
		"image-step30-768x1024":  0.098,
		"image-step30-1024x1024": 0.133,

		"image-step50-512x512":   0.028,
		"image-step50-512x768":   0.063,
		"image-step50-512x1024":  0.091,
		"image-step50-768x768":   0.112,
		"image-step50-768x1024":  0.161,
		"image-step50-1024x1024": 0.224,

		"image-step100-512x512":   0.049,
		"image-step100-512x768":   0.119,
		"image-step100-512x1024":  0.182,
		"image-step100-768x768":   0.217,
		"image-step100-768x1024":  0.315,
		"image-step100-1024x1024": 0.448,

		"image-step150-512x512":   0.07,
		"image-step150-512x768":   0.175,
		"image-step150-512x1024":  0.273,
		"image-step150-768x768":   0.322,
		"image-step150-768x1024":  0.469,
		"image-step150-1024x1024": 0.665,

		"stable-diffusion-xl-1024-v0-9-30":  0.112,
		"stable-diffusion-xl-1024-v0-9-50":  0.14,
		"stable-diffusion-xl-1024-v0-9-100": 0.217,

		"upscale-esrgan-v1-x2plus":                    0.014,
		"upscale-stable-diffusion-x4-latent-upscaler": 0.84,
	},
	"voice-recognition": {
		"whisper": 0.042,
		"tencent": 0.0024,
	},
	"translate": {
		// "youdao": 0.000048, // 每个字符，按量付费
		"youdao": 0.0, // 免费供应
	},
	"upload": {
		"qiniu": 0.1, // 每个文件，按量付费（免费供应）
	},
}

func GetOpenAIImagePrice(model string) float64 {
	return priceTables["openai"][model]
}

func GetOpenAITextPrice(model string, wordCount int64) float64 {
	return priceTables["openai"][model] * float64(wordCount) / 1000.0
}

func GetDeepAIPrice(model string) float64 {
	return priceTables["deepai"]["default"]
}

func GetLeapAIPrice(model string) float64 {
	return priceTables["leapai"]["default"]
}

func GetFromstonPrice(model string, csMode bool, width, height int64) float64 {
	return priceTables["fromston"]["default"]
}

func GetStabilityAIPrice(model string, steps int64, width, height int64) float64 {
	if model == "stable-diffusion-xl-1024-v0-9" {
		key := fmt.Sprintf("stable-diffusion-xl-1024-v0-9-%d", steps)
		if price, ok := priceTables["stabilityai"][key]; ok {
			return price
		}

		return priceTables["stabilityai"]["default"]
	}

	// 以长边为准计费
	size := width
	if height > width {
		size = height
	}

	key := fmt.Sprintf("image-step%d-%dx%d", steps, size, size)
	if price, ok := priceTables["stabilityai"][key]; ok {
		return price
	}

	return priceTables["stabilityai"]["default"]
}

func GetStabilityAIUpscalePrice(model string) float64 {

	key := fmt.Sprintf("upscale-%s", model)
	if price, ok := priceTables["stabilityai"][key]; ok {
		return price
	}

	return priceTables["stabilityai"]["default"]
}
func GetVoicePrice(model string) float64 {
	return priceTables["voice-recognition"][model]
}

func GetTranslatePrice(model string, wordCount int64) float64 {
	return priceTables["translate"][model] * float64(wordCount)
}

func GetUploadPrice() float64 {
	return priceTables["upload"]["qiniu"]
}

// PriceToCoins 价格值转换为 智慧果 数量
func PriceToCoins(price float64, serviceFeeRate float64) int64 {
	return int64(math.Ceil((price * 100) * (1 + serviceFeeRate)))
}

// 智慧果计费

func GetOpenAITextCoins(model string, wordCount int64) int64 {
	unit, ok := coinTables["openai"][model]
	if !ok {
		return PriceToCoins(GetOpenAITextPrice(model, wordCount), ServiceFeeRate)
	}

	// TODO 促销阶段，GPT-4 价格调整为 10 智慧果，满足任意截止:
	// 1. 至 2023-11-01
	// 2. 5000 美金消耗完毕
	if time.Now().Before(time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC)) && (model == "gpt-4" || model == "gpt-4-8k") {
		unit = 10
	}

	return int64(math.Ceil(float64(unit) * float64(wordCount) / 1000.0))
}

func GetOpenAITokensForCoins(model string, coins int64) int64 {
	unit, ok := coinTables["openai"][model]
	if !ok {
		return 0
	}

	return int64(math.Ceil(float64(coins) / float64(unit) * 1000.0))
}

func GetOpenAIImageCoins(model string) int64 {
	unit, ok := coinTables["openai"][model]
	if !ok {
		return PriceToCoins(GetOpenAIImagePrice(model), ServiceFeeRate)
	}

	return unit
}

func GetDeepAIImageCoins(model string) int64 {
	unit, ok := coinTables["deepai"]["default"]
	if !ok {
		return PriceToCoins(GetDeepAIPrice(model), ServiceFeeRate)
	}

	return unit
}

func GetLeapAIImageCoins(model string) int64 {
	unit, ok := coinTables["leapai"]["default"]
	if !ok {
		return PriceToCoins(GetLeapAIPrice(model), ServiceFeeRate)
	}

	return unit
}

func GetFromstonImageCoins(model string, csMode bool, width, height int64) int64 {
	unit, ok := coinTables["fromston"]["default"]
	if !ok {
		return PriceToCoins(GetFromstonPrice(model, csMode, width, height), ServiceFeeRate)
	}

	return unit
}

func GetStabilityAIImageCoins(model string, steps int64, width, height int64) int64 {
	if model == "stable-diffusion-xl-1024-v0-9" {
		key := fmt.Sprintf("stable-diffusion-xl-1024-v0-9-%d", steps)
		unit, ok := coinTables["stabilityai"][key]
		if !ok {
			return PriceToCoins(GetStabilityAIPrice(model, steps, width, height), ServiceFeeRate)
		}

		return unit
	}

	// 以长边为准计费
	size := width
	if height > width {
		size = height
	}

	key := fmt.Sprintf("image-step%d-%dx%d", steps, size, size)
	unit, ok := coinTables["stabilityai"][key]
	if !ok {
		return PriceToCoins(GetStabilityAIPrice(model, steps, width, height), ServiceFeeRate)
	}

	return unit
}

func GetStabilityAIImageUpscaleCoins(model string) int64 {
	key := fmt.Sprintf("upscale-%s", model)
	unit, ok := coinTables["stabilityai"][key]
	if !ok {
		return PriceToCoins(GetStabilityAIUpscalePrice(model), ServiceFeeRate)
	}

	return unit
}

func GetVoiceCoins(model string) int64 {
	unit, ok := coinTables["voice-recognition"][model]
	if !ok {
		return PriceToCoins(GetVoicePrice(model), ServiceFeeRate)
	}

	return unit
}

func GetTranslateCoins(model string, wordCount int64) int64 {
	unit, ok := coinTables["translate"][model]
	if !ok {
		return PriceToCoins(GetTranslatePrice(model, wordCount), ServiceFeeRate)
	}

	return unit
}

func GetUploadCoins() int64 {
	unit, ok := coinTables["upload"]["qiniu"]
	if !ok {
		return PriceToCoins(GetUploadPrice(), ServiceFeeRate)
	}

	return unit
}

// GetUnifiedImageGenCoins 统一的图片生成计费
func GetUnifiedImageGenCoins() int {
	return 20
}

func GetTextToVoiceCoins() int64 {
	return 1
}
