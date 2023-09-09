package alipay

import (
	"context"
	"fmt"
	"os"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/mylxsw/aidea-server/config"
)

type AlipayImpl struct {
	conf       *config.Config
	prodClient *alipay.Client
	devClient  *alipay.Client
}

func NewAlipay(conf *config.Config) (*AlipayImpl, error) {
	createClient := func(prod bool) (*alipay.Client, error) {
		privateKeyBytes, err := os.ReadFile(conf.AliPayAppPrivateKeyPath)
		if err != nil {
			return nil, err
		}

		client, err := alipay.NewClient("2021004101661425", string(privateKeyBytes), prod)
		if err != nil {
			return nil, err
		}

		client.SetLocation(alipay.LocationShanghai).
			SetCharset(alipay.UTF8).
			SetSignType(alipay.RSA2).
			SetNotifyUrl("https://ai-api.aicode.cc/v1/payment/callback/alipay-notify").
			SetReturnUrl("https://ai-api.aicode.cc/public/payment/alipay-return")

		alipayPublicKeyBytes, err := os.ReadFile(conf.AliPayPublicKeyPath)
		if err != nil {
			return nil, err
		}

		client.AutoVerifySign(alipayPublicKeyBytes)

		if err := client.SetCertSnByPath(
			conf.AliPayAppPublicKeyPath,
			conf.AliPayRootCertPath,
			conf.AliPayPublicKeyPath,
		); err != nil {
			return nil, err
		}

		return client, nil
	}

	prodClient, err := createClient(true)
	if err != nil {
		return nil, err
	}

	devClient, err := createClient(false)
	if err != nil {
		return nil, err
	}

	return &AlipayImpl{conf: conf, devClient: devClient, prodClient: prodClient}, nil
}

func (p TradePay) toBodyMap() gopay.BodyMap {
	bm := gopay.BodyMap{
		"out_trade_no": p.OutTradeNo,
		"total_amount": p.TotalAmount,
		"subject":      p.Subject,
	}

	if p.ProductCode != "" {
		bm.Set("product_code", p.ProductCode)
	}

	if p.Body != "" {
		bm.Set("body", p.Body)
	}

	if p.TimeExpire != "" {
		bm.Set("time_expire", p.TimeExpire)
	}

	if p.PassbackParams != "" {
		bm.Set("passback_params", p.PassbackParams)
	}

	return bm
}

func (pay *AlipayImpl) TradePay(ctx context.Context, source string, tradePay TradePay, isProd bool) (string, error) {
	switch source {
	case "app":
		return pay.TradeAppPay(ctx, tradePay, isProd)
	case "wap":
		return pay.TradeWapPay(ctx, tradePay, isProd)
	case "web":
		return pay.TradeWebPay(ctx, tradePay, isProd)
	default:
		return "", fmt.Errorf("unknown alipay source: %s", source)
	}
}

func (pay *AlipayImpl) TradeAppPay(ctx context.Context, tradeAppPay TradePay, isProd bool) (string, error) {
	if isProd {
		return pay.prodClient.TradeAppPay(ctx, tradeAppPay.toBodyMap())
	}

	return pay.devClient.TradeAppPay(ctx, tradeAppPay.toBodyMap())
}

func (pay *AlipayImpl) TradeWapPay(ctx context.Context, tradeAppPay TradePay, isProd bool) (string, error) {
	if isProd {
		return pay.prodClient.TradeWapPay(ctx, tradeAppPay.toBodyMap())
	}

	return pay.devClient.TradeWapPay(ctx, tradeAppPay.toBodyMap())
}

func (pay *AlipayImpl) TradeWebPay(ctx context.Context, tradeAppPay TradePay, isProd bool) (string, error) {
	// Web 支付方式不支持 PassbackParams
	tradeAppPay.PassbackParams = ""

	if isProd {
		return pay.prodClient.TradePagePay(ctx, tradeAppPay.toBodyMap())
	}

	return pay.devClient.TradePagePay(ctx, tradeAppPay.toBodyMap())
}

func (pay *AlipayImpl) VerifyCallbackSign(notifyBean any) (bool, error) {
	return alipay.VerifySignWithCert(pay.conf.AliPayPublicKeyPath, notifyBean)
}

func (pay *AlipayImpl) Enabled() bool {
	return true
}
