package voice

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/ai/openai"
	oai "github.com/sashabaranov/go-openai"
	"io"
	"os"
	"path/filepath"
	"time"
)

type OpenAIEngine struct {
	client   openai.Client
	savePath string
}

func NewOpenAIEngine(client openai.Client, savePath string) *OpenAIEngine {
	return &OpenAIEngine{client: client, savePath: savePath}
}

func (eng *OpenAIEngine) voiceTypeToOpenAIType(voiceType Type) oai.SpeechVoice {
	// alloy, echo, fable, onyx, nova, and shimmer
	switch voiceType {
	case TypeMale1:
		return "echo"
	case TypeFemale1:
		return "alloy"
	default:
		return "echo"
	}
}

func (eng *OpenAIEngine) Text2Voice(ctx context.Context, text string, voiceType Type) (string, error) {
	speech, err := eng.client.CreateSpeech(ctx, oai.CreateSpeechRequest{
		Model:          "tts-1",
		Input:          text,
		Voice:          eng.voiceTypeToOpenAIType(voiceType),
		ResponseFormat: oai.SpeechResponseFormatMp3,
	})
	if err != nil {
		return "", fmt.Errorf("语音合成失败: %w", err)
	}

	data, err := io.ReadAll(speech)
	if err != nil {
		return "", fmt.Errorf("读取语音流失败: %w", err)
	}

	savePath := filepath.Join(eng.savePath, fmt.Sprintf("%d-%x.mp3", time.Now().Unix(), md5.Sum(data)))
	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return "", err
	}

	return "file://" + savePath, nil
}
