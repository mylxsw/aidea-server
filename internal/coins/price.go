package coins

import (
	"math"
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

	"video": {
		"default":                  200,
		"stability-image-to-video": 200,
	},

	"openai": {
		// 1000 Token 计费
		"gpt-3.5-turbo":          3,   // valid $0.002/1K tokens -> ¥0.014/1K tokens
		"gpt-3.5-turbo-0613":     3,   // valid $0.002/1K tokens -> ¥0.014/1K tokens
		"gpt-3.5-turbo-1106":     3,   // valid $0.002/1K tokens -> ¥0.014/1K tokens
		"gpt-3.5-turbo-0125":     3,   // valid $0.0015/1K tokens -> ¥0.011/1K tokens
		"gpt-3.5-turbo-16k":      5,   // valid $0.004/1K tokens -> ¥0.028/1K tokens
		"gpt-3.5-turbo-16k-0613": 5,   // valid $0.004/1K tokens -> ¥0.028/1K tokens
		"gpt-4":                  50,  // valid $0.06/1K tokens  -> ¥0.42/1K tokens
		"gpt-4-8k":               50,  // $0.06/1K tokens        -> ¥0.42/1K tokens
		"gpt-4-32k":              100, // $0.12/1K tokens        -> ¥0.84/1K tokens
		"gpt-4-turbo-preview":    30,  // $0.03/1K tokens        -> ¥0.23/1K tokens
		"gpt-4-1106-preview":     30,  // $0.03/1K tokens        -> ¥0.23/1K tokens
		"gpt-4-0125-preview":     30,  // $0.03/1K tokens        -> ¥0.23/1K tokens
		"gpt-4-vision-preview":   30,  // $0.03/1K tokens        -> ¥0.23/1K tokens

		// Anthropic
		"claude-instant-1": 5,  // valid (input $1.63/million, output $5.51/million)  -> ¥0.039/1K tokens
		"claude-2":         25, // valid (input $11.2/million, output $32.68/million) -> ¥0.229/1K tokens
		"claude-3-opus":    75, // valid (input $15/million, output $75/million)      -> ¥0.63/1K tokens
		"claude-3-sonnet":  10, // valid (input $3/million, output $15/million)       -> ¥0.063/1K tokens
		"claude-3-haiku":   1,  // valid (input $0.25/million, output $1.25/million)  -> ¥0.0053/1K tokens

		// 国产模型

		// 百度 https://console.bce.baidu.com/qianfan/chargemanage/list
		"model_ernie_bot_turbo":             2,  // valid 文心一言 ¥0.008/1K tokens
		"model_ernie_bot":                   4,  // valid 文心一言 ¥0.012/1K tokens
		"model_ernie_bot_4":                 15, // valid 文心一言 4.0 ¥0.12/1K tokens
		"model_badiu_llama2_70b":            6,  // valid llama2 70b ¥0.044元/千tokens
		"model_baidu_llama2_7b_cn":          2,  // valid llama2 7b cn ¥0.006元/千tokens
		"model_baidu_llama2_13b":            2,  // valid llama2 7b ¥0.008元/千tokens
		"model_baidu_chatglm2_6b_32k":       2,  // valid chatglm2 6b ¥0.006/1K tokens
		"model_baidu_aquila_chat7b":         2,  // valid aquila chat7b ¥0.006/1K tokens
		"model_baidu_bloomz_7b":             2,  // valid bloomz 7b ¥0.006/1K tokens
		"model_baidu_llama2_13b_cn":         2,  // valid chat_law ¥0.006元/1K tokens
		"model_baidu_xuanyuan_70b":          5,  // valid xuanyuan 70b ¥0.035元/1K tokens
		"model_baidu_chat_law":              2,  // valid chat_law ¥0.008元/1K tokens
		"model_baidu_mixtral_8x7b_instruct": 5,  // valid mixtral 8x7b instruct ¥0.035元/1K tokens
		// 阿里 https://help.aliyun.com/zh/dashscope/developer-reference/tongyi-thousand-questions-metering-and-billing
		"qwen-v1":              1, // valid 通义千问 v1    ¥0.008/1K tokens
		"qwen-plus-v1":         3, // valid 通义千问 plus v1 ¥0.02/1K tokens
		"qwen-turbo":           1, // valid 通义千问 turbo ¥0.008/1K tokens
		"qwen-plus":            3, // valid 通义千问 plus ¥0.02/1K tokens
		"baichuan2-7b-chat-v1": 2, // valid 百川2 7b ¥0.006/1K tokens
		"qwen-7b-chat":         2, // valid 通义千问 7b ¥0.006/1K tokens
		"qwen-14b-chat":        2, // valid 通义千问 14b ¥0.008/1K tokens
		"qwen-max":             1, // 官方限时免费
		"qwen-max-longcontext": 1, // 官方限时免费
		"qwen-vl-plus":         1, // 官方限时免费

		// 讯飞星火 https://xinghuo.xfyun.cn/sparkapi
		"generalv3.5": 5, // valid 讯飞星火 v3.5  ¥0.036/1K tokens
		"generalv3":   5, // valid 讯飞星火 v3    ¥0.036/1K tokens
		"generalv2":   5, // valid 讯飞星火 v2    ¥0.036/1K tokens
		"general":     3, // valid 讯飞星火 v1.5  ¥0.018/1K tokens

		// 商汤（官方暂未公布价格）
		"nova-ptc-xl-v1": 3, // 大参数量
		"nova-ptc-xs-v1": 2, // 小参数量

		// 腾讯
		"hyllm":     3,  // valid 腾讯混元大模型 (Std) ¥0.10/1K tokens
		"hyllm_std": 3,  // valid 腾讯混元小模型 Std ¥0.01/1K tokens
		"hyllm_pro": 15, // valid 腾讯混元大模型 Pro ¥0.10/1K tokens

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
		//https://ai.google.dev/pricing?hl=zh-cn
		"gemini-pro":        3, // 临时价格
		"gemini-pro-vision": 5, // 临时价格

		// OpenRouter
		"01-ai.yi-34b-chat": 1, // valid ¥0.006/1K tokens

		// 天工 https://model-platform.tiangong.cn/pricing
		"SkyChat-MegaVerse": 2, // valid ¥0.01/1K tokens

		// 智谱 https://open.bigmodel.cn/pricing
		"glm-4":       15, // valid ¥0.1/1K tokens
		"glm-4v":      15, // valid ¥0.1/1K tokens
		"glm-3-turbo": 1,  // valid ¥0.005/1K tokens

		// 月之暗面 https://platform.moonshot.cn/pricing
		"moonshot-v1-8k":   2,  // valid ¥0.012/1K tokens
		"moonshot-v1-32k":  4,  // valid ¥0.024/1K tokens
		"moonshot-v1-128k": 10, // valid ¥0.06/1K tokens

		// 零一万物 https://platform.lingyiwanwu.com/docs
		"yi-34b-chat":      1, // valid ¥0.0025/1K tokens
		"yi-34b-chat-200k": 2, // valid ¥0.012/1K tokens
		"yi-vl-plus":       1, // valid ¥0.006/1K tokens
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

// GetOpenAITextCoins 智慧果计费
// @Deprecated
func GetOpenAITextCoins(model string, wordCount int64) int64 {
	unit, ok := coinTables["openai"][model]
	if !ok {
		return 0
	}

	return int64(math.Ceil(float64(unit) * float64(wordCount) / 1000.0))
}

type ModelInfo struct {
	ModelId     string
	InputPrice  int
	OutputPrice int
	PerReqPrice int
}

func GetTextModelCoinsDetail(model ModelInfo, inputToken, outputToken int64) (inputPrice float64, outputPrice float64, perReqPrice int64, totalPrice int64) {
	if model.OutputPrice > 0 || model.InputPrice > 0 {
		if model.InputPrice <= 0 {
			model.InputPrice = model.OutputPrice
		}

		inputPrice = float64(model.InputPrice) * float64(inputToken) / 1000.0
		outputPrice = float64(model.OutputPrice) * float64(outputToken) / 1000.0
		totalPrice = int64(math.Ceil(inputPrice+outputPrice)) + int64(model.PerReqPrice)

		return inputPrice, outputPrice, int64(model.PerReqPrice), totalPrice
	}

	totalPrice = GetOpenAITextCoins(model.ModelId, inputToken+outputToken) + int64(model.PerReqPrice)
	return 0, float64(totalPrice), int64(model.PerReqPrice), totalPrice
}

// GetTextModelCoins 获取文本模型计费，该接口对于 Input 和 Output 分开计费
func GetTextModelCoins(model ModelInfo, inputToken, outputToken int64) int64 {
	_, _, _, totalPrice := GetTextModelCoinsDetail(model, inputToken, outputToken)
	return totalPrice
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

// GetImageGenCoinsExcept 获取除了指定价格的所有图片生成模型
func GetImageGenCoinsExcept(coins int64) map[string]int64 {
	coinsTable := make(map[string]int64)
	for model, price := range coinTables["image"] {
		if price != coins {
			coinsTable[model] = price
		}
	}

	return coinsTable
}

// GetUnifiedVideoGenCoins 统一的视频生成计费
func GetUnifiedVideoGenCoins(model string) int {
	if price, ok := coinTables["video"][model]; ok {
		return int(price)
	}

	return int(coinTables["video"]["default"])
}

func GetTextToVoiceCoins(model string, wordCount int) int64 {
	if price, ok := coinTables["speech"][model]; ok {
		return int64(math.Ceil(float64(price) * float64(wordCount) / 1000.0))
	}

	return int64(math.Ceil(float64(coinTables["speech"]["default"]) * float64(wordCount) / 1000.0))
}
