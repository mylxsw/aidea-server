package alipay_test

import (
	"context"
	"testing"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/payment/alipay"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/go-utils/assert"
	"github.com/mylxsw/go-utils/must"
)

func TestAlipayApp(t *testing.T) {
	client := createClient()

	param := alipay.TradePay{
		Subject:     "测试APP支付",
		OutTradeNo:  "GZ201901301040355706100469",
		TotalAmount: "1.00",
	}

	payParam, err := client.TradeAppPay(context.Background(), param, false)
	assert.NoError(t, err)

	log.With(payParam).Infof("create trade app pay success")
}

func TestAlipayWap(t *testing.T) {
	client := createClient()

	param := alipay.TradePay{
		Subject:     "测试WAP支付",
		OutTradeNo:  "GZ201901301040355706100469",
		TotalAmount: "1.00",
	}

	payParam, err := client.TradeWapPay(context.Background(), param, false)
	assert.NoError(t, err)

	log.With(payParam).Infof("create trade app pay success")
}

func TestAlipayWeb(t *testing.T) {
	client := createClient()

	param := alipay.TradePay{
		Subject:        "测试Web支付",
		OutTradeNo:     "GZ201901301040355706100469",
		TotalAmount:    "1.00",
		Body:           "智慧果 100 个",
		PassbackParams: "abc=dfe&efg=123",
	}

	payParam, err := client.TradeWebPay(context.Background(), param, true)
	assert.NoError(t, err)

	log.With(payParam).Infof("create trade app pay success")
}

func createClient() alipay.Alipay {
	conf := config.Config{
		AliPayAppID:             "2021004101661425",
		AliPayAppPrivateKeyPath: "/Users/mylxsw/ResilioSync/AI/android/alipay/证书密钥/alipay-app-private-key.txt",
		AliPayAppPublicKeyPath:  "/Users/mylxsw/ResilioSync/AI/android/alipay/证书密钥/appCertPublicKey_2021004101661425.crt",
		AliPayPublicKeyPath:     "/Users/mylxsw/ResilioSync/AI/android/alipay/证书密钥/alipayCertPublicKey_RSA2.crt",
		AliPayRootCertPath:      "/Users/mylxsw/ResilioSync/AI/android/alipay/证书密钥/alipayRootCert.crt",
	}

	client, err := alipay.NewAlipay(&conf)
	must.NoError(err)
	return client
}
