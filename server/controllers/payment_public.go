package controllers

import (
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
)

type PaymentPublicController struct{}

func NewPaymentPublicController(resolver infra.Resolver) web.Controller {
	ctl := PaymentPublicController{}
	resolver.MustAutoWire(&ctl)

	return &ctl
}

func (p PaymentPublicController) Register(router web.Router) {
	router.Group("/payment", func(router web.Router) {
		router.Get("/alipay-return", p.AlipayReturn)
	})
}

func (p PaymentPublicController) AlipayReturn(ctx web.Context) web.Response {
	// https://ai-api.aicode.cc/public/payment/alipay-return?charset=utf-8&out_trade_no=24-c542fdf9-7c62-e67a-df7d-439d90e39f1a&method=alipay.trade.page.pay.return&total_amount=1.00&sign=eNmoTw6Vt6wfVeez1iGAB4iwj00EC3S9HiYlc6mqwh%2BKIaR9%2FtQ93vY8z7WIoSz65oCjeOA1no4FEQ%2F8sV%2BGuHEzoE6CcDXm3SP8%2B3GCT0FfqKV9KC8iS%2FkYTncqVf%2FQ4u6d1Aaoe829cYwimgdkLGgYb1TShxDR3C4%2FYTrllEafvQSdVoilUJRD4GAAr3iLeGXJtJv2thoevH5LIETWIcyFrY2EQHxU61yc%2B39W1KGyi3E6r8HsvzHAm%2Fhg7gtb6uY2WOt%2BSKumwU2WeFgYcOpfnGsmFu2hVaxN%2FjNmbSY1UNjjjQ15FBMYaP6SeAksBjgGlpLSQE6pMxO1gmo29g%3D%3D&trade_no=2023090422001404281443950985&auth_app_id=2021004101661425&version=1.0&app_id=2021004101661425&sign_type=RSA2&seller_id=2088341067502570&timestamp=2023-09-04+01%3A00%3A23
	return ctx.HTML(`<html><body onload="window.opener=null;window.close();"></body></html>`)
}
