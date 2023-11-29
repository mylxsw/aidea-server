package aliyun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	green20220302 "github.com/alibabacloud-go/green-20220302/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/str"
)

type Aliyun struct {
	conf  *config.Config
	sms   *dysmsapi.Client
	green *green20220302.Client
}

func New(conf *config.Config) *Aliyun {
	aliyun := &Aliyun{conf: conf}
	// 短信服务
	if array.In("aliyun", conf.SMSChannels) {
		client, err := aliyun.createClient(conf.AliyunAccessKeyID, conf.AliyunAccessSecret)
		if err != nil {
			log.Errorf("create aliyun client failed: %v", err)
		} else {
			aliyun.sms = client
		}

		// 额外参数检查
		if conf.AliyunSMSSign == "" {
			log.Errorf("阿里云短信发送必须通过 aliyun-smssign 配置参数指定短信签名")
		}

		if conf.AliyunSMSTemplateID == "" {
			log.Errorf("阿里云短信发送必须通过 aliyun-smstemplateid 配置参数指定短信验证码模板")
		}
	}

	// 内容安全检测
	green, err := green20220302.NewClient(&openapi.Config{
		AccessKeyId:     tea.String(conf.AliyunAccessKeyID),
		AccessKeySecret: tea.String(conf.AliyunAccessSecret),
		RegionId:        tea.String("cn-beijing"),
		Endpoint:        tea.String("green-cip.cn-beijing.aliyuncs.com"),
		ConnectTimeout:  tea.Int(3000),
		ReadTimeout:     tea.Int(6000),
	})
	if err != nil {
		log.Errorf("create aliyun green client failed: %v", err)
	} else {
		aliyun.green = green
	}

	return aliyun
}

func (a *Aliyun) createClient(accessKeyId, accessKeySecret string) (*dysmsapi.Client, error) {
	conf := &openapi.Config{
		AccessKeyId:     &accessKeyId,
		AccessKeySecret: &accessKeySecret,
	}
	// 访问的域名
	conf.Endpoint = tea.String("dysmsapi.aliyuncs.com")
	return dysmsapi.NewClient(conf)
}

// SendSMS 发送短信
func (a *Aliyun) SendSMS(ctx context.Context, templateId string, templateParams map[string]string, receivers ...string) error {
	request := &dysmsapi.SendBatchSmsRequest{}

	phones, _ := json.Marshal(receivers)
	request.SetPhoneNumberJson(string(phones))

	signNames := make([]string, len(receivers))
	for i := range receivers {
		signNames[i] = a.conf.AliyunSMSSign
	}
	signNamesJson, _ := json.Marshal(signNames)
	request.SetSignNameJson(string(signNamesJson))

	request.SetTemplateCode(templateId)

	params := make([]map[string]string, len(receivers))
	for i := range receivers {
		params[i] = templateParams
	}
	paramsJson, _ := json.Marshal(params)
	request.SetTemplateParamJson(string(paramsJson))

	resp, err := a.sms.SendBatchSms(request)
	if err != nil {
		return err
	}

	if *resp.Body.Code != "OK" {
		return fmt.Errorf("%s: %s", *resp.Body.Code, *resp.Body.Message)
	}

	return nil
}

type CheckType string

const (
	CheckTypeNickname   CheckType = "nickname_detection"
	CheckTypeChat       CheckType = "chat_detection"
	CheckTypeAIGCPrompt CheckType = "ai_art_detection"
)

type CheckResult struct {
	Safe   bool   `json:"safe"`
	Reason Reason `json:"reason"`
	Label  string `json:"label"`
}

func (res *CheckResult) IsReallyUnSafe() bool {
	return !res.Safe && res.Reason.RiskWords != ""
}

func (res *CheckResult) ReasonDetail() string {
	detail := res.Reason.RiskTips
	if res.Reason.RiskWords != "" {
		detail += fmt.Sprintf("（敏感词：%s）", res.Reason.RiskWords)
	}

	return detail
}

type Reason struct {
	RiskTips  string `json:"risk_tips"`
	RiskWords string `json:"risk_words"`
}

// ContentDetect 内容安全检测
func (a *Aliyun) ContentDetect(checkType CheckType, content string) (*CheckResult, error) {
	if a.green == nil {
		return &CheckResult{Safe: true}, nil
	}

	serviceParameters, _ := json.Marshal(
		map[string]interface{}{
			"content": content,
		},
	)

	textModerationRequest := &green20220302.TextModerationRequest{
		Service:           tea.String(string(checkType)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}

	runtime := &util.RuntimeOptions{}
	resp, err := a.green.TextModerationWithOptions(textModerationRequest, runtime)
	if err != nil {
		return nil, err
	}

	if *resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aliyun content detect failed: %s", *resp.Body)
	}

	if resp.Body.Data == nil || resp.Body.Data.Labels == nil || *resp.Body.Data.Labels == "" {
		return &CheckResult{Safe: true}, nil
	}

	label := *resp.Body.Data.Labels
	unsafeResult := CheckResult{
		Safe:  false,
		Label: label,
	}

	reasonStr := *resp.Body.Data.Reason
	if reasonStr != "" {
		log.Debugf("reason: %s", reasonStr)
		var reason Reason
		if err := json.Unmarshal([]byte(reasonStr), &reason); err != nil {
			log.Errorf("unmarshal reason failed: %v", err)
		} else {
			unsafeResult.Reason = reason
		}
	}

	// 涉政内容，全部拒绝
	if strings.Contains(label, "political_content") {
		return &unsafeResult, nil
	}

	// 涉黄内容
	if strings.Contains(label, "sexual_content") {
		tags := strings.Split(unsafeResult.Reason.RiskTips, ",")
		for _, tag := range tags {
			if str.In(tag, []string{"色情_性行为", "色情_诱导生成色情内容", "色情_严重色情"}) {
				return &unsafeResult, nil
			}
		}
	}

	// 暴力
	if strings.Contains(label, "violence") {
		tags := strings.Split(unsafeResult.Reason.RiskTips, ",")
		for _, tag := range tags {
			if str.In(tag, []string{"暴恐_诱导生成暴力血腥内容"}) {
				return &unsafeResult, nil
			}
		}
	}

	// 违禁内容
	if strings.Contains(label, "contraband") {
		tags := strings.Split(unsafeResult.Reason.RiskTips, ",")
		for _, tag := range tags {
			if str.In(tag, []string{"违禁_制假售假", "违禁_站点应用", "违禁_犯罪活动"}) {
				return &unsafeResult, nil
			}
		}
	}

	return &CheckResult{Safe: true}, nil

}
