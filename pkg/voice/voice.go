package voice

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/redis/go-redis/v9"
)

type Voice struct {
	conf   *config.Config
	rdb    *redis.Client
	engine Engine
	up     *uploader.Uploader
}

func NewVoice(conf *config.Config, rdb *redis.Client, eng Engine, up *uploader.Uploader) *Voice {
	return &Voice{conf: conf, rdb: rdb, engine: eng, up: up}
}

func (v *Voice) Text2VoiceOnlyCached(ctx context.Context, text string, voiceType Type) (string, error) {
	cacheKey := fmt.Sprintf("voice2text:%s:%x", voiceType, md5.Sum([]byte(text)))
	if rs, err := v.rdb.Get(ctx, cacheKey).Result(); err == nil {
		return rs, nil
	}

	return "", nil
}

func (v *Voice) Text2VoiceCached(ctx context.Context, text string, voiceType Type) (string, error) {
	cacheKey := fmt.Sprintf("voice2text:%s:%x", voiceType, md5.Sum([]byte(text)))
	if rs, err := v.rdb.Get(ctx, cacheKey).Result(); err == nil {
		return rs, nil
	}

	res, err := v.Text2Voice(ctx, text, voiceType)
	if err != nil {
		return "", err
	}

	if err := v.rdb.Set(ctx, cacheKey, res, 7*24*time.Hour).Err(); err != nil {
		return "", err
	}

	return res, nil
}

func (v *Voice) Text2Voice(ctx context.Context, text string, voiceType Type) (string, error) {
	voiceFile, err := v.engine.Text2Voice(ctx, text, voiceType)
	if err != nil {
		return "", fmt.Errorf("语音合成失败: %w", err)
	}

	if strings.HasPrefix(voiceFile, "http://") || strings.HasPrefix(voiceFile, "https://") {
		return voiceFile, nil
	}

	return v.up.UploadFile(ctx, 0, 14, strings.TrimPrefix(voiceFile, "file://"))
}
