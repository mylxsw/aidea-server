package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/mylxsw/aidea-server/internal/coins"
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

func (m Model) ToCoinModel() coins.ModelInfo {
	return coins.ModelInfo{
		ModelId:     m.ModelId,
		InputPrice:  m.Meta.InputPrice,
		OutputPrice: m.Meta.OutputPrice,
	}
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
			log.F(log.M{"model": ret}).Errorf("unmarshal model meta failed: %s", err)
		}

		// 没有设置输出价格，但是设置了输入价格，则输出价格与输入价格相同
		if ret.Meta.OutputPrice == 0 {
			ret.Meta.OutputPrice = ret.Meta.InputPrice
		}

		// 没有设置输入价格，但是设置了输出价格，则输入价格与输出价格相同
		if ret.Meta.InputPrice == 0 {
			ret.Meta.InputPrice = ret.Meta.OutputPrice
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
	// InputPrice 输入 Token 价格（智慧果/1K Token），为空则与 OutputPrice 相同
	InputPrice int `json:"input_price,omitempty"`
	// OutputPrice 输出 Token 价格（智慧果/1K Token）
	OutputPrice int `json:"output_price,omitempty"`
}

type ModelProvider struct {
	// 模型供应商 ID 优先从 channels 中查询模型供应商，不设置则根据 name 直接读取配置文件中固定的供应商配置
	ID int64 `json:"id,omitempty"`
	// Name 供应商名称
	Name string `json:"name,omitempty"`
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

type Channel struct {
	model.Channels
	Meta ChannelMeta `json:"meta,omitempty"`
}

type ChannelMeta struct {
	// UsingProxy 是否使用系统代理
	UsingProxy bool `json:"using_proxy,omitempty"`
	// OpenAIAzure 是否使用 OpenAI 的 Azure 服务
	OpenAIAzure bool `json:"openai_azure,omitempty"`
	// OpenAIAzureAPIVersion OpenAI Azure API 版本
	OpenAIAzureAPIVersion string `json:"openai_azure_api_version,omitempty"`
}

func NewChannel(ch model.ChannelsN) Channel {
	ret := Channel{Channels: ch.ToChannels()}
	if ret.MetaJson != "" {
		if err := json.Unmarshal([]byte(ret.MetaJson), &ret.Meta); err != nil {
			log.F(log.M{"model": ret}).Errorf("unmarshal channel meta failed: %s", err)
		}
	}

	return ret
}

// GetChannels 返回所有的渠道
func (repo *ModelRepo) GetChannels(ctx context.Context) ([]Channel, error) {
	channels, err := model.NewChannelsModel(repo.db).Get(ctx, query.Builder())
	if err != nil {
		return nil, err
	}

	return array.Map(channels, func(m model.ChannelsN, _ int) Channel {
		return NewChannel(m)
	}), nil
}

// GetChannel 返回指定的渠道
func (repo *ModelRepo) GetChannel(ctx context.Context, id int64) (*Channel, error) {
	ch, err := model.NewChannelsModel(repo.db).First(ctx, query.Builder().Where(model.FieldChannelsId, id))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	ret := NewChannel(*ch)
	return &ret, nil
}
