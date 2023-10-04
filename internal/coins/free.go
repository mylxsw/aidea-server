package coins

import (
	"strings"
	"time"

	"github.com/mylxsw/go-utils/array"
)

var freeModels = map[string]ModelWithName{
	"generalv2":                   {Model: "generalv2", Name: "讯飞星火 v2", FreeCount: 5},
	"nova-ptc-xl-v1":              {Model: "nova-ptc-xl-v1", Name: "商汤日日新（大参数量）", FreeCount: 5},
	"nova-ptc-xs-v1":              {Model: "nova-ptc-xs-v1", Name: "商汤日日新（小参数量）", FreeCount: 5},
	"model_ernie_bot_turbo":       {Model: "model_ernie_bot_turbo", Name: "文心一言 Turbo", FreeCount: 5},
	"model_baidu_bloomz_7b":       {Model: "model_baidu_bloomz_7b", Name: "Bloomz 7B", FreeCount: 5},
	"model_baidu_aquila_chat7b":   {Model: "model_baidu_aquila_chat7b", Name: "Aquila Chat 7B", FreeCount: 5},
	"model_baidu_chatglm2_6b_32k": {Model: "model_baidu_chatglm2_6b_32k", Name: "ChatGLM2 6B 32K", FreeCount: 5},
	"gpt-3.5-turbo":               {Model: "gpt-3.5-turbo", Name: "GPT 3.5 Turbo", FreeCount: 5},
	"gpt-4": {
		Model:     "gpt-4",
		Name:      "GPT 4",
		FreeCount: 3,
		// TODO 促销阶段，GPT-4 价格调整
		EndAt: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
		Info:  "活动截止至北京时间 2023-11-01 08:00:00",
	},
}

type ModelWithName struct {
	Model     string    `json:"model"`
	Name      string    `json:"name,omitempty"`
	Info      string    `json:"info,omitempty"`
	FreeCount int       `json:"-"`
	EndAt     time.Time `json:"-"`
}

// FreeModels returns all free models
func FreeModels() []ModelWithName {
	models := array.FromMap(freeModels)
	models = array.Filter(models, func(item ModelWithName, _ int) bool {
		if !item.EndAt.IsZero() {
			return item.FreeCount > 0 && item.EndAt.After(time.Now())
		}

		return item.FreeCount > 0
	})

	return array.Sort(models, func(item1, item2 ModelWithName) bool {
		return item1.Name < item2.Name
	})
}

// GetFreeModel returns the free model by model id
func GetFreeModel(modelID string) *ModelWithName {
	segs := strings.SplitN(modelID, ":", 2)
	id := segs[len(segs)-1]

	matched, ok := freeModels[id]
	if !ok {
		return nil
	}

	if matched.FreeCount <= 0 {
		return nil
	}

	if !matched.EndAt.IsZero() && matched.EndAt.Before(time.Now()) {
		return nil
	}

	return &matched
}

// IsFreeModel returns true if the model is free
func IsFreeModel(modelID string) bool {
	return GetFreeModel(modelID) != nil
}
