package coins

import (
	"strings"

	"github.com/mylxsw/go-utils/array"
)

var freeModels = map[string]string{
	"generalv2":                   "讯飞星火 v2",         // 讯飞星火 v2
	"nova-ptc-xl-v1":              "商汤日日新（大参数量）",     // 商汤 PTC XL v1
	"nova-ptc-xs-v1":              "商汤日日新（小参数量）",     // 商汤 PTC XS v1
	"model_ernie_bot_turbo":       "文心一言 Turbo",      // 文心一言 turbo
	"model_baidu_bloomz_7b":       "Bloomz 7B",       // Bloomz 7B
	"model_baidu_aquila_chat7b":   "Aquila Chat 7B",  // Aquila Chat 7B
	"model_baidu_chatglm2_6b_32k": "ChatGLM2 6B 32K", // ChatGLM2 6B 32K

	// TODO 免费至 2023-11-01 注意届时取消
	"gpt-3.5-turbo":     "GPT 3.5 Turbo",     // GPT-3.5 turbo
	"gpt-3.5-turbo-16k": "GPT 3.5 Turbo 16K", // GPT-3.5 turbo 16K
}

type ModelWithName struct {
	Model string `json:"model"`
	Name  string `json:"name"`
}

// FreeModels returns all free models
func FreeModels() []ModelWithName {
	res := make([]ModelWithName, len(freeModels))
	i := 0
	for model, name := range freeModels {
		res[i] = ModelWithName{Model: model, Name: name}
		i++
	}

	return array.Sort(res, func(item1, item2 ModelWithName) bool {
		return item1.Name < item2.Name
	})
}

// GetFreeModel returns the free model by model id
func GetFreeModel(modelID string) *ModelWithName {
	segs := strings.SplitN(modelID, ":", 2)
	id := segs[len(segs)-1]

	if freeModels[id] == "" {
		return nil
	}

	return &ModelWithName{Model: id, Name: freeModels[id]}
}

// IsFreeModel returns true if the model is free
func IsFreeModel(modelID string) bool {
	segs := strings.SplitN(modelID, ":", 2)
	id := segs[len(segs)-1]

	return freeModels[id] != ""
}
