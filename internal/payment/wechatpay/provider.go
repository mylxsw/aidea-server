package wechatpay

import (
	"context"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/glacier/infra"
)

type Provider struct{}

func (Provider) Register(binder infra.Binder) {
	binder.MustSingleton(func(conf *config.Config) WeChatPay {
		if !conf.EnableAlipay {
			return &WeChatPayFake{}
		}

		return NewWeChatPay(&Config{
			WeChatAppID:                 conf.WeChatAppID,
			WeChatPayMchID:              conf.WeChatPayMchID,
			WeChatPayCertSerialNumber:   conf.WeChatPayCertSerialNumber,
			WeChatPayCertPrivateKeyPath: conf.WeChatPayCertPrivateKeyPath,
			WeChatPayAPIv3Key:           conf.WeChatPayAPIv3Key,
		})
	})
}

type WeChatPay interface {
	NativePrepay(ctx context.Context, req PrepayRequest) (*PrepayResponse, error)
	AppPrepay(ctx context.Context, req PrepayRequest) (*PrepayResponse, error)
	SignAppPay(appID, timestamp, nocestr, prepayID string) (string, error)
}
