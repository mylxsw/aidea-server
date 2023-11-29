package voice

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
)

type Speak struct {
	XMLName xml.Name   `xml:"speak"`
	Version string     `xml:"version,attr"`
	Lang    string     `xml:"xml:lang,attr"`
	Voice   SpeakVoice `xml:"voice"`
}

type SpeakVoice struct {
	XMLName xml.Name `xml:"voice"`
	Lang    string   `xml:"xml:lang,attr"`
	Gender  string   `xml:"xml:gender,attr"`
	Name    string   `xml:"name,attr"`
	Content string   `xml:",chardata"`
}

func (speak Speak) String() string {
	data, _ := xml.Marshal(speak)
	return string(data)
}

func NewSpeak(name string, gender string, content string) Speak {
	return Speak{
		Version: "1.0",
		Lang:    "zh-CN",
		Voice: SpeakVoice{
			Lang:    "zh-CN",
			Gender:  gender,
			Name:    name,
			Content: content,
		},
	}
}

type MicrosoftSpeech struct {
	subscriptionKey string
	region          string
}

func NewMicrosoftSpeech(subscriptionKey string, region string) *MicrosoftSpeech {
	return &MicrosoftSpeech{
		subscriptionKey: subscriptionKey,
		region:          region,
	}
}

func (s *MicrosoftSpeech) TextToSpeech(ctx context.Context, speak Speak) ([]byte, error) {
	resp, err := misc.RestyClient(2).R().
		SetContext(ctx).
		SetHeader("Ocp-Apim-Subscription-Key", s.subscriptionKey).
		SetHeader("Content-Type", "application/ssml+xml").
		SetHeader("X-Microsoft-OutputFormat", "audio-24khz-48kbitrate-mono-mp3").
		SetBody(speak.String()).
		Post(fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", s.region))
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("text to speech failed: [%d] %s", resp.StatusCode(), string(resp.Body()))
	}

	return resp.Body(), nil
}
