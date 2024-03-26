package coins

import (
	"strings"
	"time"

	"github.com/mylxsw/go-utils/array"
)

var freeModels []ModelWithName

type ModelWithName struct {
	ID        int64     `json:"id,omitempty" yaml:"id,omitempty"`
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
