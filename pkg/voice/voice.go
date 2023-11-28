package voice

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	"github.com/mylxsw/aidea-server/pkg/uploader"
	oai "github.com/sashabaranov/go-openai"
	"io"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/redis/go-redis/v9"
)

type Voice struct {
	conf   *config.Config
	rdb    *redis.Client
	client openai.Client
	up     *uploader.Uploader
}

func NewVoice(conf *config.Config, rdb *redis.Client, client openai.Client, up *uploader.Uploader) *Voice {
	return &Voice{conf: conf, rdb: rdb, client: client, up: up}
}

func (v *Voice) Text2VoiceOnlyCached(ctx context.Context, voice oai.SpeechVoice, content string) (string, error) {
	cacheKey := fmt.Sprintf("voice2text:%s:%x", voice, md5.Sum([]byte(content)))
	if rs, err := v.rdb.Get(ctx, cacheKey).Result(); err == nil {
		return rs, nil
	}

	return "", nil
}

func (v *Voice) Text2VoiceCached(ctx context.Context, voice oai.SpeechVoice, content string) (string, error) {
	cacheKey := fmt.Sprintf("voice2text:%s:%x", voice, md5.Sum([]byte(content)))
	if rs, err := v.rdb.Get(ctx, cacheKey).Result(); err == nil {
		return rs, nil
	}

	res, err := v.Text2Voice(ctx, voice, content)
	if err != nil {
		return "", err
	}

	if err := v.rdb.Set(ctx, cacheKey, res, 7*24*time.Hour).Err(); err != nil {
		return "", err
	}

	return res, nil
}

func (v *Voice) Text2Voice(ctx context.Context, voice oai.SpeechVoice, content string) (string, error) {
	speech, err := v.client.CreateSpeech(ctx, oai.CreateSpeechRequest{
		Model:          "tts-1",
		Input:          content,
		Voice:          voice,
		ResponseFormat: oai.SpeechResponseFormatMp3,
	})
	if err != nil {
		return "", fmt.Errorf("语音合成失败: %w", err)
	}

	data, err := io.ReadAll(speech)
	if err != nil {
		return "", fmt.Errorf("读取语音流失败: %w", err)
	}

	return v.up.UploadStream(ctx, 0, 14, data, "mp3")
}
