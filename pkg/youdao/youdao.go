package youdao

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	uuid "github.com/hashicorp/go-uuid"
)

// Client is a youdao translate server client
type Client struct {
	serverURL string
	appID     string
	appKey    string
	client    *http.Client
}

// NewClient create a new youdao server client
func NewClient(serverURL, appID, appKey string) *Client {
	return &Client{
		serverURL: serverURL,
		appID:     appID,
		appKey:    appKey,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Translate 翻译文本
// 官方文档：https://ai.youdao.com/DOCSIRMA/html/自然语言翻译/API文档/文本翻译服务/文本翻译服务-API文档.html
func (client *Client) Translate(ctx context.Context, text string, from, to string) (*TranslateResult, error) {
	salt, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	curtime := strconv.Itoa(int(time.Now().Unix()))
	sign := client.createSign(text, salt, curtime)

	params := url.Values{}
	params.Add("q", text)
	params.Add("from", from)
	params.Add("to", to)
	params.Add("appKey", client.appID)
	params.Add("salt", salt)
	params.Add("sign", sign)
	params.Add("signType", "v3")
	params.Add("curtime", curtime)
	params.Add("strict", "true")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.serverURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: [%d] %s", resp.StatusCode, resp.Status)
	}

	var translateResp translateResponse
	if err := json.NewDecoder(resp.Body).Decode(&translateResp); err != nil {
		return nil, err
	}

	if translateResp.ErrorCode != "0" && translateResp.ErrorCode != "" {
		if msg, ok := ErrorCodes[translateResp.ErrorCode]; ok {
			return nil, errors.New(msg)
		}

		return nil, fmt.Errorf("translate failed, errorCode: %s", translateResp.ErrorCode)
	}

	res := TranslateResult{
		Result:   strings.Join(translateResp.Translation, "\n"),
		SpeakURL: translateResp.TSpeakUrl,
	}

	return &res, nil
}

// createSign 创建请求签名
func (client *Client) createSign(text, salt, curtime string) string {
	return CalculateSign(client.appID, client.appKey, text, salt, curtime)
}

func CalculateSign(appKey string, appSecret string, q string, salt string, curtime string) string {
	strSrc := appKey + getInput(q) + salt + curtime + appSecret
	return encrypt(strSrc)
}

func encrypt(strSrc string) string {
	bt := []byte(strSrc)
	bts := sha256.Sum256(bt)
	return hex.EncodeToString(bts[:])
}

func getInput(q string) string {
	str := []rune(q)
	strLen := len(str)
	if strLen <= 20 {
		return q
	} else {
		return string(str[:10]) + strconv.Itoa(strLen) + string(str[strLen-10:])
	}
}

type TranslateResult struct {
	Result   string `json:"result,omitempty"`
	SpeakURL string `json:"speak_url,omitempty"`
}

type translateResponse struct {
	// RequestID 请求 ID
	RequestID string `json:"requestId,omitempty"`
	// 错误返回码
	ErrorCode string `json:"errorCode,omitempty"`
	// 源语言
	Query string `json:"query,omitempty"`
	// Translation 翻译结果
	Translation []string `json:"translation,omitempty"`
	// TSpeakUrl 翻译结果发音地址, 翻译成功一定存在，需要应用绑定语音合成服务才能正常播放, 否则返回110错误码
	TSpeakUrl string `json:"tSpeakUrl,omitempty"`
	// SpeakUrl 源语言发音地址，翻译成功一定存在，需要应用绑定语音合成服务才能正常播放，否则返回110错误码
	SpeakUrl string `json:"speakUrl,omitempty"`
}

var Languages = map[string]string{
	"zh-CHS":  "中文",
	"zh-CHT":  "中文繁体",
	"en":      "英文",
	"ja":      "日文",
	"ko":      "韩文",
	"fr":      "法文",
	"es":      "西班牙文",
	"pt":      "葡萄牙文",
	"it":      "意大利文",
	"ru":      "俄文",
	"vi":      "越南文",
	"de":      "德文",
	"ar":      "阿拉伯文",
	"id":      "印尼文",
	"af":      "南非荷兰语",
	"bs":      "波斯尼亚语",
	"bg":      "保加利亚语",
	"yue":     "粤语",
	"ca":      "加泰隆语",
	"hr":      "克罗地亚语",
	"cs":      "捷克语",
	"da":      "丹麦语",
	"nl":      "荷兰语",
	"et":      "爱沙尼亚语",
	"fj":      "斐济语",
	"fi":      "芬兰语",
	"el":      "希腊语",
	"ht":      "海地克里奥尔语",
	"he":      "希伯来语",
	"hi":      "印地语",
	"mww":     "白苗语",
	"hu":      "匈牙利语",
	"sw":      "斯瓦希里语",
	"tlh":     "克林贡语",
	"lv":      "拉脱维亚语",
	"lt":      "立陶宛语",
	"ms":      "马来语",
	"mt":      "马耳他语",
	"no":      "挪威语",
	"fa":      "波斯语",
	"pl":      "波兰语",
	"otq":     "克雷塔罗奥托米语",
	"ro":      "罗马尼亚语",
	"sr-Cyrl": "塞尔维亚语(西里尔文)",
	"sr-Latn": "塞尔维亚语(拉丁文)",
	"sk":      "斯洛伐克语",
	"sl":      "斯洛文尼亚语",
	"sv":      "瑞典语",
	"ty":      "塔希提语",
	"th":      "泰语",
	"to":      "汤加语",
	"tr":      "土耳其语",
	"uk":      "乌克兰语",
	"ur":      "乌尔都语",
	"cy":      "威尔士语",
	"yua":     "尤卡坦玛雅语",
	"sq":      "阿尔巴尼亚语",
	"am":      "阿姆哈拉语",
	"hy":      "亚美尼亚语",
	"az":      "阿塞拜疆语",
	"bn":      "孟加拉语",
	"eu":      "巴斯克语",
	"be":      "白俄罗斯语",
	"ceb":     "宿务语",
	"co":      "科西嘉语",
	"eo":      "世界语",
	"tl":      "菲律宾语",
	"fy":      "弗里西语",
	"gl":      "加利西亚语",
	"ka":      "格鲁吉亚语",
	"gu":      "古吉拉特语",
	"ha":      "豪萨语",
	"haw":     "夏威夷语",
	"is":      "冰岛语",
	"ig":      "伊博语",
	"ga":      "爱尔兰语",
	"jw":      "爪哇语",
	"kn":      "卡纳达语",
	"kk":      "哈萨克语",
	"km":      "高棉语",
	"ku":      "库尔德语",
	"ky":      "柯尔克孜语",
	"lo":      "老挝语",
	"la":      "拉丁语",
	"lb":      "卢森堡语",
	"mk":      "马其顿语",
	"mg":      "马尔加什语",
	"ml":      "马拉雅拉姆语",
	"mi":      "毛利语",
	"mr":      "马拉地语",
	"mn":      "蒙古语",
	"my":      "缅甸语",
	"ne":      "尼泊尔语",
	"ny":      "齐切瓦语",
	"ps":      "普什图语",
	"pa":      "旁遮普语",
	"sm":      "萨摩亚语",
	"gd":      "苏格兰盖尔语",
	"st":      "塞索托语",
	"sn":      "修纳语",
	"sd":      "信德语",
	"si":      "僧伽罗语",
	"so":      "索马里语",
	"su":      "巽他语",
	"tg":      "塔吉克语",
	"ta":      "泰米尔语",
	"te":      "泰卢固语",
	"uz":      "乌兹别克语",
	"xh":      "南非科萨语",
	"yi":      "意第绪语",
	"yo":      "约鲁巴语",
	"zu":      "南非祖鲁语",
	"auto":    "自动识别",
}

const (
	LanguageAuto          = "auto"
	LanguageChineseSimple = "zh-CHS"
	LanguageEnglish       = "en"
)

var ErrorCodes = map[string]string{
	"101":   "缺少必填的参数，首先确保必填参数齐全，然后确认参数书写是否正确",
	"102":   "不支持的语言类型",
	"103":   "翻译文本过长",
	"104":   "不支持的API类型",
	"105":   "不支持的签名类型",
	"106":   "不支持的响应类型",
	"107":   "不支持的传输加密类型",
	"108":   "应用ID无效，注册账号，登录后台创建应用并完成绑定，可获得应用ID和应用密钥等信息",
	"109":   "batchLog格式不正确",
	"110":   "无相关服务的有效应用,应用没有绑定服务应用，可以新建服务应用。注：某些服务的翻译结果发音需要tts服务，需要在控制台创建语音合成服务绑定应用后方能使用。",
	"111":   "开发者账号无效",
	"112":   "请求服务无效",
	"113":   "q不能为空",
	"114":   "不支持的图片传输方式",
	"116":   "strict字段取值无效，请参考文档填写正确参数值",
	"201":   "解密失败，可能为DES,BASE64,URLDecode的错误",
	"202":   "签名检验失败,如果确认应用ID和应用密钥的正确性，仍返回202，一般是编码问题。请确保翻译文本 q 为UTF-8编码.",
	"203":   "访问IP地址不在可访问IP列表",
	"205":   "请求的接口与应用的平台类型不一致，确保接入方式（Android SDK、IOS SDK、API）与创建的应用平台类型一致。如有疑问请参考入门指南",
	"206":   "因为时间戳无效导致签名校验失败",
	"207":   "重放请求",
	"301":   "辞典查询失败",
	"302":   "翻译查询失败",
	"303":   "服务端的其它异常",
	"304":   "会话闲置太久超时",
	"308":   "rejectFallback参数错误",
	"309":   "domain参数错误",
	"310":   "未开通领域翻译服务",
	"401":   "账户已经欠费，请进行账户充值",
	"402":   "offlinesdk不可用",
	"411":   "访问频率受限,请稍后访问",
	"412":   "长请求过于频繁，请稍后访问",
	"1001":  "无效的OCR类型",
	"1002":  "不支持的OCR image类型",
	"1003":  "不支持的OCR Language类型",
	"1004":  "识别图片过大",
	"1201":  "图片base64解密失败",
	"1301":  "OCR段落识别失败",
	"1411":  "访问频率受限",
	"1412":  "超过最大识别字节数",
	"2003":  "不支持的语言识别Language类型",
	"2004":  "合成字符过长",
	"2005":  "不支持的音频文件类型",
	"2006":  "不支持的发音类型",
	"2201":  "解密失败",
	"2301":  "服务的异常",
	"2411":  "访问频率受限,请稍后访问",
	"2412":  "超过最大请求字符数",
	"3001":  "不支持的语音格式",
	"3002":  "不支持的语音采样率",
	"3003":  "不支持的语音声道",
	"3004":  "不支持的语音上传类型",
	"3005":  "不支持的语言类型",
	"3006":  "不支持的识别类型",
	"3007":  "识别音频文件过大",
	"3008":  "识别音频时长过长",
	"3009":  "不支持的音频文件类型",
	"3010":  "不支持的发音类型",
	"3201":  "解密失败",
	"3301":  "语音识别失败",
	"3302":  "语音翻译失败",
	"3303":  "服务的异常",
	"3411":  "访问频率受限,请稍后访问",
	"3412":  "超过最大请求字符数",
	"4001":  "不支持的语音识别格式",
	"4002":  "不支持的语音识别采样率",
	"4003":  "不支持的语音识别声道",
	"4004":  "不支持的语音上传类型",
	"4005":  "不支持的语言类型",
	"4006":  "识别音频文件过大",
	"4007":  "识别音频时长过长",
	"4201":  "解密失败",
	"4301":  "语音识别失败",
	"4303":  "服务的异常",
	"4411":  "访问频率受限,请稍后访问",
	"4412":  "超过最大请求时长",
	"5001":  "无效的OCR类型",
	"5002":  "不支持的OCR image类型",
	"5003":  "不支持的语言类型",
	"5004":  "识别图片过大",
	"5005":  "不支持的图片类型",
	"5006":  "文件为空",
	"5201":  "解密错误，图片base64解密失败",
	"5301":  "OCR段落识别失败",
	"5411":  "访问频率受限",
	"5412":  "超过最大识别流量",
	"9001":  "不支持的语音格式",
	"9002":  "不支持的语音采样率",
	"9003":  "不支持的语音声道",
	"9004":  "不支持的语音上传类型",
	"9005":  "不支持的语音识别 Language类型",
	"9301":  "ASR识别失败",
	"9303":  "服务器内部错误",
	"9411":  "访问频率受限（超过最大调用次数）",
	"9412":  "超过最大处理语音长度",
	"10001": "无效的OCR类型",
	"10002": "不支持的OCR image类型",
	"10004": "识别图片过大",
	"10201": "图片base64解密失败",
	"10301": "OCR段落识别失败",
	"10411": "访问频率受限",
	"10412": "超过最大识别流量",
	"11001": "不支持的语音识别格式",
	"11002": "不支持的语音识别采样率",
	"11003": "不支持的语音识别声道",
	"11004": "不支持的语音上传类型",
	"11005": "不支持的语言类型",
	"11006": "识别音频文件过大",
	"11007": "识别音频时长过长，最大支持30s",
	"11201": "解密失败",
	"11301": "语音识别失败",
	"11303": "服务的异常",
	"11411": "访问频率受限,请稍后访问",
	"11412": "超过最大请求时长",
	"12001": "图片尺寸过大",
	"12002": "图片base64解密失败",
	"12003": "引擎服务器返回错误",
	"12004": "图片为空",
	"12005": "不支持的识别图片类型",
	"12006": "图片无匹配结果",
	"13001": "不支持的角度类型",
	"13002": "不支持的文件类型",
	"13003": "表格识别图片过大",
	"13004": "文件为空",
	"13301": "表格识别失败",
	"15001": "需要图片",
	"15002": "图片过大（1M）",
	"15003": "服务调用失败",
	"17001": "需要图片",
	"17002": "图片过大（1M）",
	"17003": "识别类型未找到",
	"17004": "不支持的识别类型",
	"17005": "服务调用失败",
}
