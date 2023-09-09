package alipay

import (
	"context"
)

type Alipay interface {
	TradePay(ctx context.Context, source string, tradePay TradePay, isProd bool) (string, error)
	TradeAppPay(ctx context.Context, tradeAppPay TradePay, isProd bool) (string, error)
	TradeWapPay(ctx context.Context, tradeAppPay TradePay, isProd bool) (string, error)
	TradeWebPay(ctx context.Context, tradeAppPay TradePay, isProd bool) (string, error)
	VerifyCallbackSign(notifyBean any) (bool, error)
	Enabled() bool
}

type TradePay struct {
	// OutTradeNo 商户订单号。
	// 由商家自定义，64个字符以内，仅支持字母、数字、下划线且需保证在商户端不重复
	OutTradeNo string `json:"out_trade_no,omitempty"`
	// TotalAmount 订单总金额。
	// 单位为元，精确到小数点后两位，取值范围：[0.01,100000000]
	TotalAmount string `json:"total_amount,omitempty"`
	// Subject 订单标题
	Subject string `json:"subject,omitempty"`

	// 以下为可选参数

	// ProductCode 产品码。
	// 商家和支付宝签约的产品码。 枚举值（点击查看签约情况）：
	// QUICK_MSECURITY_PAY：无线快捷支付产品；
	// CYCLE_PAY_AUTH：周期扣款产品。
	// 默认值为QUICK_MSECURITY_PAY。
	ProductCode string `json:"product_code,omitempty"`
	// Body 订单附加信息。
	// 如果请求时传递了该参数，将在异步通知、对账单中原样返回，同时会在商户和用户的pc账单详情中作为交易描述展示
	Body string `json:"body,omitempty"`
	// TimeExpire 订单绝对超时时间。
	// 格式为yyyy-MM-dd HH:mm:ss。
	// 注：time_expire和timeout_express两者只需传入一个或者都不传，如果两者都传，优先使用time_expire。
	TimeExpire string `json:"time_expire,omitempty"`
	// PassbackParams 公用回传参数。
	// 如果请求时传递了该参数，支付宝会在异步通知时将该参数原样返回。
	// 本参数必须进行UrlEncode之后才可以发送给支付宝。
	PassbackParams string `json:"passback_params,omitempty"`

	// Web/Wap 专用
	ReturnURL string `json:"return_url,omitempty"`
}
