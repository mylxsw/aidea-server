package coins

import (
	"strings"
	"time"

	"github.com/mylxsw/go-utils/array"
)

var freeModels = []ModelWithName{
	{Model: "generalv2", Name: "讯飞星火 v2", FreeCount: 5},
	{Model: "nova-ptc-xl-v1", Name: "商汤日日新（大参数量）", FreeCount: 5},
	{Model: "nova-ptc-xs-v1", Name: "商汤日日新（小参数量）", FreeCount: 5},
	{Model: "model_ernie_bot_turbo", Name: "文心一言 Turbo", FreeCount: 5},
	{Model: "model_baidu_bloomz_7b", Name: "Bloomz 7B", FreeCount: 5},
	{Model: "model_baidu_aquila_chat7b", Name: "Aquila Chat 7B", FreeCount: 5},
	{Model: "model_baidu_chatglm2_6b_32k", Name: "ChatGLM2 6B 32K", FreeCount: 5},
	{Model: "Baichuan2-53B", Name: "百川 53B", FreeCount: 5},
	{Model: "360GPT_S2_V9", Name: "360 智脑", FreeCount: 5},
	{Model: "gpt-3.5-turbo", Name: "GPT 3.5 Turbo", FreeCount: 5, NonCN: true},
	{
		Model:     "gpt-4",
		Name:      "GPT 4",
		FreeCount: 3,
		// TODO 促销阶段，GPT-4 价格调整
		EndAt: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
		Info:  "活动截止至北京时间 2023-11-01 08:00:00",
		NonCN: true,
	},
	{Model: "gpt-3.5-turbo", Name: "南贤", FreeCount: 5},
	{
		Model:     "gpt-4",
		Name:      "北丑",
		FreeCount: 3,
		// TODO 促销阶段，GPT-4 价格调整
		EndAt: time.Date(2023, 11, 1, 0, 0, 0, 0, time.UTC),
		Info:  "活动截止至北京时间 2023-11-01 08:00:00",
	},
}

type ModelWithName struct {
	Model     string    `json:"model" yaml:"model"`
	Name      string    `json:"name,omitempty" yaml:"name,omitempty"`
	Info      string    `json:"info,omitempty" yaml:"info,omitempty"`
	FreeCount int       `json:"free_count,omitempty" yaml:"free_count"`
	EndAt     time.Time `json:"end_at,omitempty" yaml:"end_at,omitempty"`
	NonCN     bool      `json:"non_cn,omitempty" yaml:"non_cn,omitempty"`
}

// FreeModels returns all free models
func FreeModels() []ModelWithName {
	models := array.Filter(freeModels, func(item ModelWithName, _ int) bool {
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

	var matched ModelWithName
	for _, model := range freeModels {
		if model.Model == id {
			matched = model
			break
		}
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
