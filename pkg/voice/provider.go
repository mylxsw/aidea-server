package voice

import (
	"context"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"github.com/mylxsw/glacier/infra"
	"github.com/redis/go-redis/v9"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config, rdb *redis.Client, openaiClient openai.Client, up *uploader.Uploader) *Voice {
		var engine Engine
		switch conf.TextToVoiceEngine {
		case "azure":
			engine = NewAzureVoiceEngine(conf.TextToVoiceAzureKey, conf.TextToVoiceAzureRegion, conf.TempDir)
		case "minimax":
			engine = NewMiniMaxVoiceClient(conf.MiniMaxAPIKey, conf.MiniMaxGroupID)
		default:
			engine = NewOpenAIEngine(openaiClient, conf.TempDir)
		}

		return NewVoice(conf, rdb, engine, up)
	})
}

// Type 语音音色类型
type Type string

const (
	TypeMale1   Type = "male1"
	TypeFemale1 Type = "female1"
)

// Engine 接口
type Engine interface {
	Text2Voice(ctx context.Context, text string, voiceType Type) (string, error)
}
