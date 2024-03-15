package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/go-utils/array"
)

type ModelRepo struct {
	db *sql.DB
}

func NewModelRepo(db *sql.DB) *ModelRepo {
	return &ModelRepo{db: db}
}

type Model struct {
	model.Models
	Meta      ModelMeta       `json:"meta,omitempty"`
	Providers []ModelProvider `json:"providers,omitempty"`
}

func (m Model) SelectProvider() ModelProvider {
	if len(m.Providers) == 0 {
		return ModelProvider{Name: "openai"}
	}

	// TODO 更好的选择策略
	return m.Providers[0]
}

const (
	ModelStatusEnabled  int64 = 1
	ModelStatusDisabled int64 = 0
)

func NewModel(m model.Models) Model {
	ret := Model{Models: m}

	if ret.ShortName == "" {
		ret.ShortName = ret.Name
	}

	if ret.MetaJson != "" {
		if err := json.Unmarshal([]byte(ret.MetaJson), &ret.Meta); err != nil {
			log.F(log.M{"model": ret}).Errorf("unmarshal model abilities failed: %s", err)
		}
	}

	if ret.ProvidersJson != "" {
		if err := json.Unmarshal([]byte(ret.ProvidersJson), &ret.Providers); err != nil {
			log.F(log.M{"model": ret}).Errorf("unmarshal model providers failed: %s", err)
		}
	}

	return ret
}

type ModelMeta struct {
	// Vision 是否支持视觉能力
	Vision bool `json:"vision,omitempty"`
	// Restricted 是否是受限制的模型
	Restricted bool `json:"restricted,omitempty"`
	// MaxContext 最大上下文长度
	MaxContext int `json:"max_context,omitempty"`
}

type ModelProvider struct {
	// Name 供应商名称
	Name string `json:"name"`
	// ModelRewrite 模型名称重写，如果为空，则使用模型的名称
	ModelRewrite string `json:"model_rewrite,omitempty"`
	// Prompt 全局的系统提示语
	Prompt string `json:"prompt,omitempty"`
}

// SupportProvider check if the model support the provider
func (m Model) SupportProvider(providerName string) *ModelProvider {
	for _, p := range m.Providers {
		if p.Name == providerName {
			return &p
		}
	}

	return nil
}

// GetModels return all models
func (repo *ModelRepo) GetModels(ctx context.Context) ([]Model, error) {
	models, err := model.NewModelsModel(repo.db).Get(ctx, query.Builder())
	if err != nil {
		return nil, err
	}

	return array.Map(models, func(m model.ModelsN, _ int) Model {
		return NewModel(m.ToModels())
	}), nil
}

// GetModel return model by modelID
func (repo *ModelRepo) GetModel(ctx context.Context, modelID string) (*Model, error) {
	m, err := model.NewModelsModel(repo.db).First(ctx, query.Builder().Where(model.FieldModelsModelId, modelID))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	ret := NewModel(m.ToModels())
	return &ret, nil
}
