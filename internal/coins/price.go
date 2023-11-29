package coins

import (
	"math"
	"time"
)

var coinTables = map[string]CoinTable{
	// 统一图片价格
	"image": {
		"default": 20,
		// 1024×1024           -> $0.040
		// 1024×1792,1792×1024 -> $0.080
		"dall-e-3": 50,
		// 1024×1024           -> $0.080
		// 1024×1792,1792×1024 -> $0.120
		"dall-e-3:hd": 80,
		// 1024x1024 -> $0.020
		// 512x512   -> $0.018
		// 256x256   -> $0.016
		"dall-e-2": 20,
	},

	"openai": {
		// 1000 Token 计费
		"gpt-3.5-turbo":          3,   // valid $0.002/1K tokens -> ¥0.014/1K tokens
		"gpt-3.5-turbo-0613":     3,   // valid $0.002/1K tokens -> ¥0.014/1K tokens
		"gpt-3.5-turbo-1106":     3,   // valid $0.002/1K tokens -> ¥0.014/1K tokens
		"gpt-3.5-turbo-16k":      5,   // valid $0.004/1K tokens -> ¥0.028/1K tokens
		"gpt-3.5-turbo-16k-0613": 5,   // valid $0.004/1K tokens -> ¥0.028/1K tokens
		"gpt-4":                  50,  // valid $0.06/1K tokens  -> ¥0.42/1K tokens
		"gpt-4-8k":               50,  // $0.06/1K tokens        -> ¥0.42/1K tokens
		"gpt-4-32k":              100, // $0.12/1K tokens        -> ¥0.84/1K tokens
		"gpt-4-1106-preview":     30,  // $0.03/1K tokens        -> ¥0.23/1K tokens
		"gpt-4-vision-preview":   30,  // $0.03/1K tokens        -> ¥0.23/1K tokens

		// Anthropic
		"claude-instant-1": 5,  // valid (input $1.63/million, output $5.51/million)  -> ¥0.039/1K tokens
		"claude-2":         25, // valid (input $11.2/million, output $32.68/million) -> ¥0.229/1K tokens

		// 国产模型

		// 百度 https://console.bce.baidu.com/qianfan/chargemanage/list
		"model_ernie_bot_turbo":       2,  // valid 文心一言 ¥0.008/1K tokens
		"model_ernie_bot":             4,  // valid 文心一言 ¥0.012/1K tokens
		"model_ernie_bot_4":           15, // valid 文心一言 4.0 ¥0.12/1K tokens
		"model_badiu_llama2_70b":      6,  // valid llama2 70b ¥0.044元/千tokens
		"model_baidu_llama2_7b_cn":    2,  // valid llama2 7b cn ¥0.006元/千tokens
		"model_baidu_llama2_13b":      2,  // valid llama2 7b ¥0.008元/千tokens
		"model_baidu_chatglm2_6b_32k": 2,  // valid chatglm2 6b ¥0.006/1K tokens
		"model_baidu_aquila_chat7b":   2,  // valid aquila chat7b ¥0.006/1K tokens
		"model_baidu_bloomz_7b":       2,  // valid bloomz 7b ¥0.006/1K tokens

		// 阿里 https://help.aliyun.com/zh/dashscope/developer-reference/tongyi-thousand-questions-metering-and-billing
		"qwen-v1":              1, // valid 通义千问 v1    ¥0.008/1K tokens
		"qwen-plus-v1":         3, // valid 通义千问 plus v1 ¥0.02/1K tokens
		"qwen-turbo":           1, // valid 通义千问 turbo ¥0.008/1K tokens
		"qwen-plus":            3, // valid 通义千问 plus ¥0.02/1K tokens
		"baichuan2-7b-chat-v1": 2, // valid 百川2 7b ¥0.006/1K tokens
		"qwen-7b-chat":         2, // valid 通义千问 7b ¥0.006/1K tokens
		"qwen-14b-chat":        2, // valid 通义千问 14b ¥0.008/1K tokens

		// 讯飞星火 https://xinghuo.xfyun.cn/sparkapi
		"generalv3": 5, // valid 讯飞星火 v3    ¥0.036/1K tokens
		"generalv2": 5, // valid 讯飞星火 v2    ¥0.036/1K tokens
		"general":   3, // valid 讯飞星火 v1.5  ¥0.018/1K tokens

		// 商汤（官方暂未公布价格）
		"nova-ptc-xl-v1": 3, // 大参数量
		"nova-ptc-xs-v1": 2, // 小参数量

		// 腾讯
		"hyllm": 15, // valid 腾讯混元大模型 ¥0.10/1K tokens

		// 百川 https://platform.baichuan-ai.com/price
		"Baichuan2-53B": 3, // valid 百川 53b ¥0.02/1K tokens

		// 360 智脑
		"360GPT_S2_V9": 2, // valid 360 智脑 ¥0.012/1K tokens

		// OneAPI
		// ChatGLM: https://open.bigmodel.cn/pricing
		"chatglm_turbo": 1, // valid ¥0.005/1K tokens
		"chatglm_pro":   2, // valid ¥0.01/1K tokens
		"chatglm_std":   1, // valid ¥0.005/1K tokens
		"chatglm_lite":  1, // valid ¥0.004/1K tokens
		// Google
		"PaLM-2": 3, // valid ¥0.0148/1K tokens
	},

	"voice-recognition": {
		"tencent": 1, // valid
	},

	"speech": {
		"default": 15,

		// 1K 字符 0.15 元
		"tts-1":    15,
		"tts-1-hd": 30,
	},

	"translate": {
		"youdao": 0,
	},

	"upload": {
		"qiniu": 1,
	},
}

func GetCoinsTable() map[string]CoinTable {
	return coinTables
}

// PriceToCoins 价格值转换为 智慧果 数量
func PriceToCoins(price float64, serviceFeeRate float64) int64 {
	return int64(math.Ceil((price * 100) * (1 + serviceFeeRate)))
}

// 智慧果计费

func GetOpenAITextCoins(model string, wordCount int64) int64 {
	unit, ok := coinTables["openai"][model]
	if !ok {
		return 50
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

func GetVoiceCoins(model string) int64 {
	unit, ok := coinTables["voice-recognition"][model]
	if !ok {
		return 0
	}

	return unit
}

func GetTranslateCoins(model string, wordCount int64) int64 {
	unit, ok := coinTables["translate"][model]
	if !ok {
		return 0
	}

	return unit
}

func GetUploadCoins() int64 {
	unit, ok := coinTables["upload"]["qiniu"]
	if !ok {
		return 0
	}

	return unit
}

// GetUnifiedImageGenCoins 统一的图片生成计费
func GetUnifiedImageGenCoins(model string) int {
	if price, ok := coinTables["image"][model]; ok {
		return int(price)
	}

	return int(coinTables["image"]["default"])
}

func GetTextToVoiceCoins(model string, wordCount int) int64 {
	if price, ok := coinTables["speech"][model]; ok {
		return int64(math.Ceil(float64(price) * float64(wordCount) / 1000.0))
	}

	return int64(math.Ceil(float64(coinTables["speech"]["default"]) * float64(wordCount) / 1000.0))
}
