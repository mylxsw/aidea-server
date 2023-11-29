package billing

import (
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
)

type BillingController struct {
}

func NewBillingController(resolver infra.Resolver) web.Controller {
	ctl := &BillingController{}
	resolver.MustAutoWire(ctl)
	return ctl
}

func (ctl *BillingController) Register(router web.Router) {
	router.Group("/billing", func(router web.Router) {
		router.Get("/subscription", ctl.Subscription)
	})
}

type OpenAISubscriptionResponse struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	AccessUntil        int64   `json:"access_until"`
}

func (ctl *BillingController) Subscription(ctx web.Context) web.Response {
	return ctx.JSON(OpenAISubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       10000,
		HardLimitUSD:       10000,
		SystemHardLimitUSD: 10000,
		AccessUntil:        0,
	})
}
