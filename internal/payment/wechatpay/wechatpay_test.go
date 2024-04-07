package wechatpay

import (
	"context"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/go-utils/assert"
	"os"
	"testing"
)

func TestWeChatPay_Prepay(t *testing.T) {
	conf := Config{
		WeChatAppID:                 os.Getenv("WECHAT_APPID"),
		WeChatPayMchID:              os.Getenv("WECHAT_MCHID"),
		WeChatPayCertSerialNumber:   os.Getenv("WECHAT_CERT_SERIAL_NUMBER"),
		WeChatPayCertPrivateKeyPath: os.Getenv("WECHAT_CERT_PK_PATH"),
		WeChatPayAPIv3Key:           os.Getenv("WECHAT_APIV3_KEY"),
	}

	wechatPay := NewWeChatPay(&conf)
	res, err := wechatPay.AppPrepay(context.TODO(), PrepayRequest{
		OutTradeNo:  misc.PaymentID(1000000000),
		Description: "test",
		NotifyURL:   "https://example.com/notify",
		Amount:      100,
	})
	assert.NoError(t, err)

	t.Logf("%+v", res)
}
