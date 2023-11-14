package voice

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mylxsw/aidea-server/config"
	qiniuAuth "github.com/qiniu/go-sdk/v7/auth"
	"github.com/redis/go-redis/v9"
)

type Voice struct {
	conf *config.Config
	rdb  *redis.Client
}

func NewVoice(conf *config.Config, rdb *redis.Client) *Voice {
	return &Voice{conf: conf, rdb: rdb}
}

type VoiceRequest struct {
	// Spkid TTS 发音人标识音源 id 7-14,实际可用范围根据情况, 可以不设置,默认是 7; 其中
	// 7:精品女声，成熟，声音柔和纯美;
	// 8:精品女声，西安方言;
	// 9:精品女声，东北方言;
	// 10:精品男声，成熟正式，播音腔;
	// 11:精品男声，男孩，活泼开朗;
	// 12:精品男声，常见解说配音腔;
	// 13:精品男声，央视新闻播音腔;
	// 14:精品女声，少女音色。
	Spkid int64 `json:"spkid,omitempty"`
	// Content 需要进行语音合成的文本内容，最短1个字，最长200字
	Content string `json:"content"`
	// AudioType 可不填，不填时默认为 3。
	// audioType=3 返回 16K 采样率的 mp3
	// audioType=4 返回 8K 采样率的 mp3
	// audioType=5 返回 24K 采样率的 mp3
	// audioType=6 返回 48k采样率的mp3
	// audioType=7 返回 16K 采样率的 pcm 格式
	// audioType=8 返回 8K 采样率的 pcm 格式
	// audioType=9 返回 24k 采样率的pcm格式
	// audioType=10 返回 8K 采样率的 wav 格式
	// audioType=11 返回 16K 采样率的 wav 格式
	AudioType int64 `json:"audioType,omitempty"`
	// Volume 音量大小，取值范围为 0.75 - 1.25，默认为1
	Volume float64 `json:"volume,omitempty"`
	// Speed 语速，取值范围为 0.75 - 1.25，默认为1
	Speed float64 `json:"speed,omitempty"`
}

type VoiceResponse struct {
	Code   string `json:"code"`
	Msg    string `json:"msg"`
	Result struct {
		AudioUrl string `json:"audioUrl"`
	} `json:"result"`
}

func (v *Voice) Text2VoiceOnlyCached(ctx context.Context, spkid int64, content string) (string, error) {
	cacheKey := fmt.Sprintf("voice2text:%d:%x", spkid, md5.Sum([]byte(content)))
	if rs, err := v.rdb.Get(ctx, cacheKey).Result(); err == nil {
		return rs, nil
	}

	return "", nil
}

func (v *Voice) Text2VoiceCached(ctx context.Context, spkid int64, content string) (string, error) {
	cacheKey := fmt.Sprintf("voice2text:%d:%x", spkid, md5.Sum([]byte(content)))
	if rs, err := v.rdb.Get(ctx, cacheKey).Result(); err == nil {
		return rs, nil
	}

	res, err := v.Text2Voice(ctx, VoiceRequest{
		Spkid:   spkid,
		Content: content,
	})
	if err != nil {
		return "", err
	}

	if err := v.rdb.Set(ctx, cacheKey, res, 7*24*time.Hour).Err(); err != nil {
		return "", err
	}

	return res, nil
}

func (v *Voice) Text2Voice(ctx context.Context, voice VoiceRequest) (string, error) {
	mac := qiniuAuth.New(v.conf.StorageAppKey, v.conf.StorageAppSecret)

	client := &http.Client{}

	reqData, err := json.Marshal(voice)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://ap-gate-z0.qiniuapi.com/voice/v2/tts", bytes.NewReader(reqData))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/json")
	token, err := mac.SignRequestV2(req)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Qiniu %s", token))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed: %s", resp.Status)
	}

	defer resp.Body.Close()

	var respData VoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", err
	}

	if respData.Code != "0" {
		return "", fmt.Errorf("request failed: %s", respData.Msg)
	}

	return respData.Result.AudioUrl, nil
}
