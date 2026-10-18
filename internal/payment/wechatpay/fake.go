package wechatpay

import (
	"context"
	"errors"

	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

type WeChatPayFake struct {
}

func (w *WeChatPayFake) NativePrepay(ctx context.Context, req PrepayRequest) (*PrepayResponse, error) {
	panic("implement me")
}

func (w *WeChatPayFake) AppPrepay(ctx context.Context, req PrepayRequest) (*PrepayResponse, error) {
	panic("implement me")
}

func (w *WeChatPayFake) SignAppPay(appID, timestamp, nocestr, prepayID string) (string, error) {
	panic("implement me")
}

func (w *WeChatPayFake) QueryNative(ctx context.Context, outTradeNo string) (*payments.Transaction, error) {
	return nil, errors.New("wechat pay disabled")
}

func (w *WeChatPayFake) CloseNative(ctx context.Context, outTradeNo string) error {
	return errors.New("wechat pay disabled")
}
