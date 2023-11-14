package tencent_test

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/tencent"
	"os"
	"testing"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/go-utils/assert"
)

func TestVoiceToText(t *testing.T) {
	conf := &config.Config{
		TencentSecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
		TencentSecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
	}

	c := tencent.NewTencent(conf)
	res, err := c.VoiceToText(context.TODO(), "/Users/mylxsw/Downloads/龙洲大道.m4a")
	assert.NoError(t, err)
	t.Log(res)
}

func TestSendSMS(t *testing.T) {
	conf := &config.Config{
		TencentSecretID:    os.Getenv("TENCENTCLOUD_SECRET_ID"),
		TencentSecretKey:   os.Getenv("TENCENTCLOUD_SECRET_KEY"),
		TencentSMSSDKAppID: "1400827805",
		SMSChannels:        []string{"tencent"},
	}

	c := tencent.NewTencent(conf)
	assert.NoError(t, c.SendSMS(context.TODO(), "1822196", []string{"123456"}, "18888888888"))
}
