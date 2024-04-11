package wechatpay

import "context"

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
