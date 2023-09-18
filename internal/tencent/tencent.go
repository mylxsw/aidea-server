package tencent

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/array"
	asr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/asr/v20190614"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

type Tencent struct {
	voice *asr.Client
	sms   *sms.Client
	conf  *config.Config
}

func NewTencent(conf *config.Config) *Tencent {
	tencent := &Tencent{conf: conf}
	if conf.TencentSecretID != "" && conf.TencentSecretKey != "" {
		if conf.UseTencentVoiceToText {
			asrClient, err := tencent.createVoiceClient()
			if err != nil {
				log.Errorf("create tencent voice client failed: %s", err)
			} else {
				tencent.voice = asrClient
			}
		}

		if array.In("tencent", conf.SMSChannels) {
			smsClient, err := tencent.createSMSClient()
			if err != nil {
				log.Errorf("create tencent sms client failed: %s", err)
			} else {
				tencent.sms = smsClient
			}

			// 额外参数检查
			if conf.TencentSMSSign == "" {
				log.Errorf("腾讯云短信发送必须通过 tencent-smssign 配置参数指定短信签名")
			}

			if conf.TencentSMSTemplateID == "" {
				log.Errorf("腾讯云短信发送必须通过 tencent-smstemplateid 配置参数指定短信验证码模板")
			}
		}
	}

	return tencent
}

func (t *Tencent) createSMSClient() (*sms.Client, error) {
	credential := common.NewCredential(
		t.conf.TencentSecretID,
		t.conf.TencentSecretKey,
	)

	prof := profile.NewClientProfile()
	return sms.NewClient(credential, "ap-beijing", prof)
}

func (t *Tencent) createVoiceClient() (*asr.Client, error) {
	credential := common.NewCredential(
		t.conf.TencentSecretID,
		t.conf.TencentSecretKey,
	)

	prof := profile.NewClientProfile()
	return asr.NewClient(credential, "ap-shanghai", prof)
}

// SendSMS 发送短信
func (t *Tencent) SendSMS(ctx context.Context, templateID string, templateParams []string, receivers ...string) error {
	req := sms.NewSendSmsRequest()
	req.SmsSdkAppId = common.StringPtr(t.conf.TencentSMSSDKAppID)
	req.SignName = common.StringPtr(t.conf.TencentSMSSign)
	req.TemplateId = common.StringPtr(templateID)
	req.TemplateParamSet = common.StringPtrs(templateParams)
	req.PhoneNumberSet = common.StringPtrs(receivers)

	_, err := t.sms.SendSmsWithContext(ctx, req)
	return err
}

// VoiceToText 语音转文字
func (t *Tencent) VoiceToText(ctx context.Context, filename string) (string, error) {

	fileData, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	data := base64.StdEncoding.EncodeToString(fileData)

	req := asr.NewSentenceRecognitionRequest()
	req.EngSerViceType = common.StringPtr("16k_zh_dialect")
	req.SourceType = common.Uint64Ptr(1)
	req.VoiceFormat = common.StringPtr("m4a")
	req.Data = common.StringPtr(data)
	req.DataLen = common.Int64Ptr(int64(len(data)))

	resp, err := t.voice.SentenceRecognitionWithContext(ctx, req)
	if err != nil {
		return "", err
	}

	return *resp.Response.Result, nil
}
