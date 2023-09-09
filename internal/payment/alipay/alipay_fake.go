package alipay

import "context"

type AlipayFake struct {
}

func (pay *AlipayFake) TradePay(ctx context.Context, source string, tradePay TradePay, isProd bool) (string, error) {
	panic("implement me")
}

func (pay *AlipayFake) TradeAppPay(ctx context.Context, tradeAppPay TradePay, isProd bool) (string, error) {
	panic("implement me")
}

func (pay *AlipayFake) TradeWapPay(ctx context.Context, tradeAppPay TradePay, isProd bool) (string, error) {
	panic("implement me")
}

func (pay *AlipayFake) TradeWebPay(ctx context.Context, tradeAppPay TradePay, isProd bool) (string, error) {
	panic("implement me")
}

func (pay *AlipayFake) VerifyCallbackSign(notifyBean any) (bool, error) {
	panic("implement me")
}

func (pay *AlipayFake) Enabled() bool {
	return false
}
