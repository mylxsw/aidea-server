package aliyun_test

import (
	"context"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/aliyun"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
)

func TestSendSMS(t *testing.T) {
	ali := aliyun.New(&config.Config{
		AliyunAccessKeyID:  os.Getenv("ALIYUN_ACCESSKEY"),
		AliyunAccessSecret: os.Getenv("ALIYUN_SECRET"),
		SMSChannels:        []string{"aliyun"},
	})

	assert.NoError(t, ali.SendSMS(context.TODO(), "SMS_279297328", map[string]string{"code": "123456"}, "18678859721"))
}

func TestContentDetect(t *testing.T) {
	ali := aliyun.New(&config.Config{
		AliyunAccessKeyID:  os.Getenv("ALIYUN_ACCESSKEY"),
		AliyunAccessSecret: os.Getenv("ALIYUN_SECRET"),
	})

	rs, err := ali.ContentDetect(aliyun.CheckTypeAIGCPrompt, "assistant")
	assert.NoError(t, err)

	log.With(rs).Info("result")
}
