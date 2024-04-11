package voice

import (
	"context"
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"os"
	"path/filepath"
	"time"
)

type AzureSpeak struct {
	XMLName xml.Name        `xml:"speak"`
	Version string          `xml:"version,attr"`
	Lang    string          `xml:"xml:lang,attr"`
	Voice   AzureSpeakVoice `xml:"voice"`
}

type AzureSpeakVoice struct {
	XMLName xml.Name `xml:"voice"`
	Lang    string   `xml:"xml:lang,attr"`
	Gender  string   `xml:"xml:gender,attr"`
	Name    string   `xml:"name,attr"`
	Content string   `xml:",chardata"`
}

func (speak AzureSpeak) String() string {
	data, _ := xml.Marshal(speak)
	return string(data)
}

func NewAzureSpeak(name string, gender string, content string) AzureSpeak {
	return AzureSpeak{
		Version: "1.0",
		Lang:    "zh-CN",
		Voice: AzureSpeakVoice{
			Lang:    "zh-CN",
			Gender:  gender,
			Name:    name,
			Content: content,
		},
	}
}

type AzureVoiceEngine struct {
	subscriptionKey string
	region          string
	savePath        string
}

func NewAzureVoiceEngine(subscriptionKey string, region string, savePath string) *AzureVoiceEngine {
	return &AzureVoiceEngine{
		subscriptionKey: subscriptionKey,
		region:          region,
		savePath:        savePath,
	}
}

func (s *AzureVoiceEngine) voiceTypeToAzureVoiceType(voiceType Type) (name string, gender string) {
	switch voiceType {
	case TypeMale1:
		return "zh-CN-YunyangNeural", "Male"
	case TypeFemale1:
		return "zh-CN-XiaochenNeural", "Female"
	default:
		return "zh-CN-XiaochenNeural", "Female"
	}
}

func (s *AzureVoiceEngine) Text2Voice(ctx context.Context, text string, voiceType Type) (string, error) {
	voiceName, voiceGender := s.voiceTypeToAzureVoiceType(voiceType)
	speak := NewAzureSpeak(
		voiceName,
		voiceGender,
		text,
	)
	resp, err := misc.RestyClient(2).R().
		SetContext(ctx).
		SetHeader("Ocp-Apim-Subscription-Key", s.subscriptionKey).
		SetHeader("Content-Type", "application/ssml+xml").
		SetHeader("X-Microsoft-OutputFormat", "audio-24khz-48kbitrate-mono-mp3").
		SetBody(speak.String()).
		Post(fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", s.region))
	if err != nil {
		return "", err
	}

	if !resp.IsSuccess() {
		return "", fmt.Errorf("text to speech failed: [%d] %s", resp.StatusCode(), string(resp.Body()))
	}

	savePath := filepath.Join(s.savePath, fmt.Sprintf("%d-%x.mp3", time.Now().Unix(), md5.Sum([]byte(speak.String()))))
	if err := os.WriteFile(savePath, resp.Body(), 0644); err != nil {
		return "", err
	}

	return "file://" + savePath, nil
}
