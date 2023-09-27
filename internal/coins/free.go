package coins

import (
	"github.com/mylxsw/go-utils/array"
	"strings"
)

var freeModels = []string{
	"generalv2",                   // 讯飞星火 v2
	"nova-ptc-xl-v1",              // 商汤 PTC XL v1
	"nova-ptc-xs-v1",              // 商汤 PTC XS v1
	"model_ernie_bot_turbo",       // 文心一言 turbo
	"model_baidu_bloomz_7b",       // Bloomz 7B
	"model_baidu_aquila_chat7b",   // Aquila Chat 7B
	"model_baidu_chatglm2_6b_32k", // ChatGLM2 6B 32K

	// TODO 免费至 2023-11-01 注意届时取消
	"gpt-3.5-turbo",          // GPT-3.5 turbo
	"gpt-3.5-turbo-0613",     // GPT-3.5 turbo 0613
	"gpt-3.5-turbo-16k",      // GPT-3.5 turbo 16K
	"gpt-3.5-turbo-16k-0613", // GPT-3.5 turbo 16K 0613
}

// IsFreeModel returns true if the model is free
func IsFreeModel(modelID string) bool {
	segs := strings.SplitN(modelID, ":", 2)
	id := segs[len(segs)-1]

	return array.In(id, freeModels)
}
