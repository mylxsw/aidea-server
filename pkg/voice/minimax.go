package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/pkg/misc"
)

type MiniMaxVoiceClient struct {
	apiKey  string // API key for the MiniMax API
	groupID string // Group ID for the MiniMax API
}

func NewMiniMaxVoiceClient(apiKey string, groupID string) *MiniMaxVoiceClient {
	return &MiniMaxVoiceClient{apiKey: apiKey, groupID: groupID}
}

type MiniMaxVoiceRequest struct {
	// Model 模型版本 speech-01、speech-02
	Model string `json:"model"`
	// VoiceID 请求的音色编号
	// 支持系统音色(id)以及复刻音色（id）两种类型，其中系统音色（ID）如下：
	// - 青涩青年音色：male-qn-qingse
	// - 精英青年音色：male-qn-jingying
	// - 霸道青年音色：male-qn-badao
	// - 青年大学生音色：male-qn-daxuesheng
	// - 少女音色：female-shaonv
	// - 御姐音色：female-yujie
	// - 成熟女性音色：female-chengshu
	// - 甜美女性音色：female-tianmei
	// - 男性主持人：presenter_male
	// - 女性主持人：presenter_female
	// - 男性有声书1：audiobook_male_1
	// - 男性有声书2：audiobook_male_2
	// - 女性有声书1：audiobook_female_1
	// - 女性有声书2：audiobook_female_2
	// - 青涩青年音色-beta：male-qn-qingse-jingpin
	// - 精英青年音色-beta：male-qn-jingying-jingpin
	// - 霸道青年音色-beta：male-qn-badao-jingpin
	// - 青年大学生音色-beta：male-qn-daxuesheng-jingpin
	// - 少女音色-beta：female-shaonv-jingpin
	// - 御姐音色-beta：female-yujie-jingpin
	// - 成熟女性音色-beta：female-chengshu-jingpin
	// - 甜美女性音色-beta：female-tianmei-jingpin
	VoiceID string `json:"voice_id,omitempty"`
	// TimerWeights 音色相关信息
	TimerWeights []TimerWeight `json:"timer_weights,omitempty"`
	// Speed 生成声音的语速，取值范围为0.5-2.0，默认为1.0, 取值越大，语速越快
	Speed float64 `json:"speed,omitempty"`
	// Vol 生成声音的音量，默认值为1.0，取值越大，音量越高，范围（0, 10]
	Vol int `json:"vol,omitempty"`
	// OutputFormat 生成声音的音频格式, 可选，默认值为mp3，可选范围：mp3、wav、pcm、flac、aac
	OutputFormat string `json:"output_format,omitempty"`
	// Pitch 生成声音的语调, 可选，默认值为0（0为原音色输出，取值需为整数），范围[-12, 12]
	Pitch int `json:"pitch,omitempty"`
	// Text 需要生成的文本
	// 长度限制<50000字符（如需要控制语音中间隔时间，在字间增加<#x#>,x单位为秒，支持0.01-99.99s，最多两位小数）
	// 支持自定义文本与文本之间的语音时间间隔，以实现自定义文本语音停顿时间的效果。需要注意的是文本间隔时间需设置在
	// 两个可以语音发音的文本之间，且不能设置多个连续的时间间隔
	Text string `json:"text"`
	// AudioSampleRate 生成声音的音频采样率，默认值为 32000，可选范围：16000、24000、32000
	AudioSampleRate int `json:"audio_sample_rate,omitempty"`
	// Bitrate 生成声音的比特率，范围[32000, 64000，128000]，默认值为 128000
	Bitrate int `json:"bitrate,omitempty"`
	// CharToPitch 替换需要特殊标注的文字、符号及对应的注音
	// 功能类型1，替换声调：["燕少飞/(yan4)(shao3)(fei1)"]
	// 功能类型2，替换字符：["omg/oh my god","=/等于"]
	// 声调用数字代替，一声（阴平）为1，二声（阳平）为2，三声（上声）为3，四声（去声）为4），轻声为5
	CharToPitch []string `json:"char_to_pitch,omitempty"`
}

type TimerWeight struct {
	// VoiceID 请求的音色编号
	VoiceID string `json:"voice_id"`
	// Weight 权重，最多支持4种音色混合，取值为整数，单一音色取值占比越高，合成音色越像；取值范围为1-100
	Weight int `json:"weight"`
}

type MiniMaxVoiceResponse struct {
	// AudioFile 生成的音频文件地址
	AudioFile string `json:"audio_file"`
	// TraceID 用于在咨询/反馈时帮助定位问题
	TraceID string `json:"trace_id"`
	// SubtitleFile 合成的字幕下载链接
	SubtitleFile string    `json:"subtitle_file"`
	ExtraInfo    ExtraInfo `json:"extra_info"`

	BaseResp BaseResp `json:"base_resp,omitempty"`
}

type ExtraInfo struct {
	// AudioLength 音频时长，精确到毫秒
	AudioLength int `json:"audio_length"`
	// AudioSize 默认为24000，如客户请求参数进行调整，会根据请求参数生成
	AudioSampleRate int `json:"audio_sample_rate"`
	// AudioSize 单位为字节
	AudioSize int `json:"audio_size"`
	// Bitrate 默认为168000，如客户请求参数进行调整，会根据请求参数生成
	Bitrate int `json:"bitrate"`
	// WordCount 已经发音的字数统计（不算标点等其他符号，包含汉字数字字母）
	WordCount int `json:"word_count"`
	// InvisibleCharacterRatio 非法字符不超过10%（包含10%），音频会正常生成并返回非法字符占比；最大不超过0.1（10%），超过进行报错
	InvisibleCharacterRatio float64 `json:"invisible_character_ratio"`
	// UsageCharacters 本次语音生成的计费字符数
	UsageCharacters int `json:"usage_characters"`
}

type BaseResp struct {
	// StatusCode
	// 1000，未知错误
	// 1001，超时
	// 1002，触发限流
	// 1004，鉴权失败
	// 1013，服务内部错误及非法字符超过10%
	// 2013，输入格式信息不正常
	StatusCode int `json:"status_code"`
	// StatusMessage 错误详情
	StatusMessage string `json:"status_msg"`
}

// TextToSpeech converts text to speech using the MiniMax API
func (c *MiniMaxVoiceClient) TextToSpeech(ctx context.Context, req MiniMaxVoiceRequest) (*MiniMaxVoiceResponse, error) {
	resp, err := misc.RestyClient(2).R().SetContext(ctx).
		SetHeader("Authorization", "Bearer "+c.apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		Post("https://api.minimax.chat/v1/t2a_pro?GroupId=" + c.groupID)
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("text to speech failed: [%d] %s", resp.StatusCode(), string(resp.Body()))
	}

	var result MiniMaxVoiceResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, err
	}

	if result.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("text to speech failed: [%d] %s", result.BaseResp.StatusCode, result.BaseResp.StatusMessage)
	}

	return &result, nil
}

func (c *MiniMaxVoiceClient) voiceTypeToVoiceID(voiceType Type) string {
	switch voiceType {
	case TypeMale1:
		return "presenter_male"
	case TypeFemale1:
		return "presenter_female"
	default:
		return "presenter_male"
	}
}

func (c *MiniMaxVoiceClient) Text2Voice(ctx context.Context, text string, voiceType Type) (string, error) {
	req := MiniMaxVoiceRequest{
		Model:           "speech-02",
		VoiceID:         c.voiceTypeToVoiceID(voiceType),
		Text:            text,
		OutputFormat:    "mp3",
		AudioSampleRate: 24000,
		Bitrate:         128000,
		Speed:           0.9,
	}
	resp, err := c.TextToSpeech(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.AudioFile, nil
}
