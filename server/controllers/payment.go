package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/internal/payment/wechatpay"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"github.com/mylxsw/aidea-server/pkg/youdao"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/ephemeralkey"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/webhook"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/payment/applepay"

	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/payment/alipay"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/server/auth"
	"github.com/mylxsw/aidea-server/server/controllers/common"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
	"github.com/mylxsw/go-utils/ternary"
)

type PaymentController struct {
	translater youdao.Translater   `autowire:"@"`
	queue      *queue.Queue        `autowire:"@"`
	payRepo    *repo.PaymentRepo   `autowire:"@"`
	alipay     alipay.Alipay       `autowire:"@"`
	applepay   applepay.ApplePay   `autowire:"@"`
	wechatpay  wechatpay.WeChatPay `autowire:"@"`
	conf       *config.Config      `autowire:"@"`
}

func NewPaymentController(resolver infra.Resolver) web.Controller {
	ctl := PaymentController{}
	resolver.MustAutoWire(&ctl)

	return &ctl
}

func (ctl *PaymentController) Register(router web.Router) {
	router.Group("/payment", func(router web.Router) {
		// 可以公开访问的可购买产品列表
		router.Get("/products", ctl.AppleProducts)
		router.Get("/others/products", ctl.AppleProducts) // @deprecated(since 1.0.9)
		router.Get("/apple/products", ctl.AppleProducts)  // @deprecated(since 1.0.9)
		router.Get("/alipay/products", ctl.AppleProducts) // @deprecated(since 1.0.8)

		// Apple 应用内支付
		router.Post("/apple", ctl.CreateApplePayment)
		router.Put("/apple/{id}", ctl.UpdateApplePayment)
		router.Delete("/apple/{id}", ctl.CancelApplePayment)
		router.Post("/apple/{id}/verify", ctl.VerifyApplePayment)

		// 支付宝支付 @deprecated(since 1.0.8)
		router.Group("/alipay", func(router web.Router) {
			router.Post("/", ctl.CreateAlipay)
			router.Post("/client-confirm", ctl.AlipayClientConfirm)
		})

		// 支付宝支付别名(IOS 完全去支付宝，避免审核被拒)
		router.Group("/others", func(router web.Router) {
			router.Post("/", ctl.CreateAlipay)
			router.Post("/client-confirm", ctl.AlipayClientConfirm)
		})

		// Stripe 支付
		router.Group("/stripe", func(router web.Router) {
			router.Post("/payment-sheet", ctl.CreateStripePayment)
		})

		// 微信支付
		router.Group("/wechatpay", func(router web.Router) {
			router.Post("/", ctl.CreateWechatPayment)
		})

		// 支付状态查询
		router.Group("/status", func(router web.Router) {
			router.Get("/{id}", ctl.QueryPaymentStatus)
		})

		// 支付结果回调通知
		router.Group("/callback", func(router web.Router) {
			router.Post("/alipay-notify", ctl.AlipayNotify)
			router.Any("/stripe/webhook", ctl.StripeWebhook)
			router.Any("/wechat-pay/notify", ctl.WechatPayNotify)
		})

	})
}

// CreateAlipay 发起支付宝付款
func (ctl *PaymentController) CreateAlipay(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	if !ctl.alipay.Enabled() {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "支付宝支付功能尚未开启"), http.StatusBadRequest)
	}

	productId := webCtx.Input("product_id")
	if productId == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	source := webCtx.Input("source")
	if source == "" {
		source = "app"
	}

	if !array.In(source, []string{"app", "web", "wap"}) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	product := coins.GetProduct(productId)
	if product == nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	if !array.In("alipay", product.GetSupportMethods()) {
		log.F(log.M{"user_id": user.ID}).Errorf("product %s not support alipay", productId)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	paymentID, err := ctl.payRepo.CreateAliPayment(ctx, user.ID, productId, source)
	if err != nil {
		log.WithFields(log.Fields{
			"err":        err.Error(),
			"product_id": productId,
			"user_id":    user.ID,
			"source":     source,
		}).Error("create payment failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	passbackParams := url.Values{}

	passbackParams.Add("user_id", strconv.Itoa(int(user.ID)))
	passbackParams.Add("product_id", productId)
	passbackParams.Add("payment_id", paymentID)

	// 用于测试专用：设置所有支付付款金额为 1 分钱
	// product.RetailPrice = 1

	trade := alipay.TradePay{
		Subject:        product.Name,
		TotalAmount:    fmt.Sprintf("%.2f", float64(product.RetailPrice)/100.0),
		OutTradeNo:     paymentID,
		Body:           product.Name,
		PassbackParams: passbackParams.Encode(),
	}

	log.With(trade).Debugf("create alipay payment")

	payParams, err := ctl.alipay.TradePay(ctx, source, trade, !ctl.conf.AlipaySandbox)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("create alipay payment failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"params": payParams, "payment_id": paymentID, "sandbox": ctl.conf.AlipaySandbox})
}

type AlipayClientConfirm struct {
	SignType                  string `json:"sign_type"`
	Sign                      string `json:"sign"`
	AlipayTradeAppPayResponse struct {
		AppID       string `json:"app_id"`
		AuthAppID   string `json:"auth_app_id"`
		Charset     string `json:"charset"`
		Code        string `json:"code"`
		Msg         string `json:"msg"`
		OutTradeNo  string `json:"out_trade_no"`
		SellerID    string `json:"seller_id"`
		Timestamp   string `json:"timestamp"`
		TotalAmount string `json:"total_amount"`
		TradeNo     string `json:"trade_no"`
	} `json:"alipay_trade_app_pay_response"`
}

// AlipayClientConfirm 支付宝支付结果确认（客户端，只有 App 会使用到）
func (ctl *PaymentController) AlipayClientConfirm(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	if !ctl.alipay.Enabled() {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "支付宝支付功能尚未开启"), http.StatusBadRequest)
	}

	resultStatus := webCtx.Input("resultStatus")
	result := webCtx.Input("result")
	memo := webCtx.Input("memo")
	extendInfo := webCtx.Input("extendInfo")

	var res AlipayClientConfirm
	if err := json.Unmarshal([]byte(result), &res); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("parse alipay client confirm failed")
	}

	log.WithFields(log.Fields{
		"resultStatus":  resultStatus,
		"result":        result,
		"memo":          memo,
		"extendInfo":    extendInfo,
		"result_parsed": res,
	}).Debugf("alipay client confirm")

	his, err := ctl.payRepo.GetPaymentHistory(ctx, res.AlipayTradeAppPayResponse.OutTradeNo)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
		}

		log.WithFields(log.Fields{"err": err}).Error("get payment history failed")
	}

	if his.Status != int(repo.PaymentStatusSuccess) {
		log.WithFields(log.Fields{
			"his":    his,
			"result": res,
		}).Warning("客户端获取支付宝支付状态失败，支付状态不是成功状态")
	}

	return webCtx.JSON(web.M{
		"status": "ok",
	})
}

// QueryPaymentStatus 查询支付状态
func (ctl *PaymentController) QueryPaymentStatus(ctx context.Context, webCtx web.Context) web.Response {
	paymentId := webCtx.PathVar("id")
	history, err := ctl.payRepo.GetPaymentHistory(ctx, paymentId)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrNotFound), http.StatusNotFound)
		}

		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	var note string
	switch history.Status {
	case repo.PaymentStatusWaiting:
		note = "等待支付"
	case repo.PaymentStatusSuccess:
		note = "支付成功"
	case repo.PaymentStatusFailed:
		note = "支付失败"
	case repo.PaymentStatusCanceled:
		note = "支付已取消"
	}

	return webCtx.JSON(web.M{
		"success": history.Status == repo.PaymentStatusSuccess,
		"note":    note,
	})
}

func priceStrToInt64Penny(price string) int64 {
	priceFloat, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return 0
	}

	return int64(priceFloat * 100)
}

// AlipayNotify 支付宝支付结果服务端回调通知 https://opendocs.alipay.com/open/204/105301
func (ctl *PaymentController) AlipayNotify(ctx context.Context, webCtx web.Context) web.Response {
	log.WithFields(log.Fields{
		"body": string(webCtx.Body()),
	}).Info("alipay callback")

	if !ctl.alipay.Enabled() {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "支付宝支付功能尚未开启"), http.StatusBadRequest)
	}

	sign := webCtx.Input("sign")
	notifyId := webCtx.Input("notify_id")
	notifyType := webCtx.Input("notify_type")
	tradeStatus := webCtx.Input("trade_status")
	receiptAmount := webCtx.Input("receipt_amount")
	appId := webCtx.Input("app_id")
	buyerPayAmount := webCtx.Input("buyer_pay_amount")
	totalAmount := webCtx.Input("total_amount")
	pointAmount := webCtx.Input("point_amount")
	tradeNo := webCtx.Input("trade_no")
	outTradeNo := webCtx.Input("out_trade_no")
	buyerId := webCtx.Input("buyer_id")
	buyerLogonId := webCtx.Input("buyer_logon_id")

	his, err := ctl.payRepo.GetAlipayHistory(ctx, outTradeNo)
	if err != nil {
		log.WithFields(log.Fields{
			"body":  string(webCtx.Body()),
			"error": err.Error(),
		}).Info("alipay callback invalid, payment not found")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	userId, paymentId, productId := his.UserId, his.PaymentId, his.ProductId

	params := make(map[string]interface{})
	// 注意：这里的 PostForm 必须在调用 webCtx.Input 等方法之后才能使用（ParseForm）
	for k, v := range webCtx.Request().Raw().PostForm {
		params[k] = v[0]
	}

	log.With(params).Debugf("alipay callback params")

	signOk, err := ctl.alipay.VerifyCallbackSign(params)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("verify alipay callback sign failed")
	}

	log.WithFields(log.Fields{
		"sign":             sign,
		"sign_ok":          signOk,
		"notify_id":        notifyId,
		"notify_type":      notifyType,
		"receipt_amount":   receiptAmount,
		"trade_status":     tradeStatus,
		"app_id":           appId,
		"buyer_pay_amount": buyerPayAmount,
		"total_amount":     totalAmount,
		"trade_no":         tradeNo,
		"out_trade_no":     outTradeNo,
		"buyer_id":         buyerId,
		"buyer_logon_id":   buyerLogonId,
		"payment_id":       paymentId,
		"product_id":       productId,
		"user_id":          userId,
		"point_amount":     pointAmount,
	}).Infof("alipay callback success")

	var status int64
	var note string
	if tradeStatus == "TRADE_SUCCESS" {
		status = int64(repo.PaymentStatusSuccess)
	} else {
		status = int64(repo.PaymentStatusFailed)
		switch tradeStatus {
		case "WAIT_BUYER_PAY":
			note = "交易创建，等待买家付款"
		case "TRADE_CLOSED":
			note = "未付款交易超时关闭，或支付完成后全额退款"
		case "TRADE_FINISHED":
			note = "交易结束，不可退款"
		}
	}

	product := coins.GetProduct(productId)

	env := ternary.If(ctl.conf.AlipaySandbox, "Sandbox", "Production")
	aliPay := repo.AlipayPayment{
		ProductID:      productId,
		BuyerID:        buyerId,
		InvoiceAmount:  priceStrToInt64Penny(receiptAmount),
		ReceiptAmount:  priceStrToInt64Penny(receiptAmount),
		BuyerPayAmount: priceStrToInt64Penny(buyerPayAmount),
		TotalAmount:    priceStrToInt64Penny(totalAmount),
		PointAmount:    priceStrToInt64Penny(pointAmount),
		TradeNo:        tradeNo,
		BuyerLogonID:   buyerLogonId,
		PurchaseAt:     time.Now(),
		Status:         status,
		Environment:    env,
		Note:           note,
	}
	eventID, err := ctl.payRepo.CompleteAliPayment(ctx, int64(userId), paymentId, aliPay)
	if err != nil {
		// 如果已经处理过了，直接返回成功
		if errors.Is(err, repo.ErrPaymentHasBeenProcessed) {
			return webCtx.Raw(func(w http.ResponseWriter) {
				_, _ = w.Write([]byte("success"))
			})
		}

		log.WithFields(log.Fields{
			"err":        err.Error(),
			"payment_id": paymentId,
		}).Error("complete payment failed")

		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if eventID > 0 {
		payload := queue.PaymentPayload{
			UserID:    int64(userId),
			ProductID: productId,
			PaymentID: paymentId,
			Note:      product.Name,
			Source:    "alipay-purchase",
			Env:       env,
			CreatedAt: time.Now(),
			EventID:   eventID,
		}

		if _, err := ctl.queue.Enqueue(&payload, queue.NewPaymentTask); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("enqueue payment task failed")
		}
	}

	return webCtx.Raw(func(w http.ResponseWriter) {
		_, _ = w.Write([]byte("success"))
	})
}

// AppleProducts 支付产品清单
func (ctl *PaymentController) AppleProducts(ctx context.Context, webCtx web.Context, client *auth.ClientInfo) web.Response {
	products := array.Map(coins.Products, func(product coins.Product, _ int) coins.Product {
		product.ExpirePolicyText = product.GetExpirePolicyText()
		if product.RetailPrice == 0 {
			product.RetailPrice = product.Quota
		}

		if product.RetailPriceUSD <= 0 {
			product.RetailPriceUSD = product.GetRetailPriceUSD()
		}

		return product
	})

	products = array.Filter(products, func(prod coins.Product, _ int) bool {
		if prod.PlatformLimit == "" {
			return true
		}

		if prod.PlatformLimit == coins.PlatformNoneIOS && client.IsIOS() {
			return false
		}

		if prod.PlatformLimit == coins.PlatformIOS && !client.IsIOS() {
			return false
		}

		return true
	})

	return webCtx.JSON(web.M{
		"prefer_usd": coins.PreferUSD,
		"consume":    products,
		"note": `
1. 您购买的智慧果需在有效期内使用，逾期未使用即失效；
2. 智慧果不支持退款、提现或转赠他人；
3. 支付如遇到问题，可发邮件至 support@aicode.cc，我们会为您解决。
		`,
	})
}

// CreateApplePayment 发起 Apple 应用内支付
func (ctl *PaymentController) CreateApplePayment(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	if !ctl.applepay.Enabled() {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "Apple 应用内支付功能尚未开启"), http.StatusBadRequest)
	}

	productId := webCtx.Input("product_id")
	if productId == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	if !coins.IsProduct(productId) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	paymentID, err := ctl.payRepo.CreateApplePayment(ctx, user.ID, productId)
	if err != nil {
		log.WithFields(log.Fields{
			"err":        err.Error(),
			"product_id": productId,
			"user_id":    user.ID,
		}).Error("create payment failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	log.WithFields(log.Fields{
		"payment_id": paymentID,
		"product_id": productId,
	}).Info("create apple payment")

	return webCtx.JSON(map[string]interface{}{
		"id":         paymentID,
		"product_id": productId,
	})
}

// UpdateApplePayment  更新 Apple 应用内支付状态
func (ctl *PaymentController) UpdateApplePayment(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	if !ctl.applepay.Enabled() {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "Apple 应用内支付功能尚未开启"), http.StatusBadRequest)
	}

	paymentId := webCtx.PathVar("id")
	serverVerifyData := webCtx.Input("server_verify_data")
	verifyDataSource := webCtx.Input("verify_data_source")

	log.WithFields(log.Fields{
		"payment_id":         paymentId,
		"server_verify_data": serverVerifyData,
		"verify_data_source": verifyDataSource,
	}).Info("update apple payment")

	if err := ctl.payRepo.UpdateApplePayment(ctx, user.ID, paymentId, verifyDataSource, serverVerifyData); err != nil {
		log.WithFields(log.Fields{
			"err":        err.Error(),
			"payment_id": paymentId,
			"user_id":    user.ID,
		}).Error("update payment failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(map[string]interface{}{
		"status": "ok",
		"id":     paymentId,
	})
}

// VerifyApplePayment 验证 Apple 应用内支付结果
func (ctl *PaymentController) VerifyApplePayment(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	if !ctl.applepay.Enabled() {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "Apple 应用内支付功能尚未开启"), http.StatusBadRequest)
	}

	paymentId := webCtx.PathVar("id")

	productId := webCtx.Input("product_id")
	purchaseId := webCtx.Input("purchase_id")
	transactionDate := webCtx.Input("transaction_date")
	serverVerifyData := webCtx.Input("server_verify_data")
	verifyDataSource := webCtx.Input("verify_data_source")
	status := webCtx.Input("status")

	log.WithFields(log.Fields{
		"payment_id":         paymentId,
		"product_id":         productId,
		"purchase_id":        purchaseId,
		"transaction_date":   transactionDate,
		"server_verify_data": serverVerifyData,
		"verify_data_source": verifyDataSource,
		"status":             status,
	}).Info("verify apple payment")

	applePayment, resp, inApp, err := ctl.applepay.VerifyPayment(ctx, purchaseId, serverVerifyData)
	if err != nil {
		log.WithFields(log.Fields{
			"err":           err.Error(),
			"payment_id":    paymentId,
			"apple_payment": applePayment,
		}).Error("verify payment failed")

		return webCtx.JSONError(common.Text(webCtx, ctl.translater, err.Error()), http.StatusInternalServerError)
	}

	if resp.Status != 0 {
		log.WithFields(log.Fields{
			"payment_id": paymentId,
			"verify":     resp,
		}).Error("verify payment failed")

		applePayment.Status = int64(repo.PaymentStatusFailed)
		if _, err := ctl.payRepo.CompleteApplePayment(ctx, user.ID, paymentId, applePayment); err != nil {
			log.WithFields(log.Fields{
				"err":           err.Error(),
				"apple_payment": applePayment,
				"payment_id":    paymentId,
			}).Error("complete payment failed")
		}

		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	receiptProductId := inApp.ProductID
	if receiptProductId != productId {
		log.WithFields(log.Fields{
			"payment_id":         paymentId,
			"verify":             resp,
			"product_id":         productId,
			"receipt_product_id": receiptProductId,
		}).Error("verify payment: product id not match")
	}

	applePayment.Status = int64(repo.PaymentStatusSuccess)
	eventID, err := ctl.payRepo.CompleteApplePayment(ctx, user.ID, paymentId, applePayment)
	if err != nil {
		log.WithFields(log.Fields{
			"err":           err.Error(),
			"apple_payment": applePayment,
			"payment_id":    paymentId,
		}).Error("complete payment failed")

		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	product := coins.GetProduct(receiptProductId)
	payload := queue.PaymentPayload{
		UserID:    user.ID,
		Email:     user.Email,
		ProductID: receiptProductId,
		PaymentID: paymentId,
		Note:      product.Name,
		Source:    "apple-purchase",
		Env:       string(resp.Environment),
		CreatedAt: time.Now(),
		EventID:   eventID,
	}

	if _, err := ctl.queue.Enqueue(&payload, queue.NewPaymentTask); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("enqueue payment task failed")
	}

	return webCtx.JSON(map[string]interface{}{
		"status":  "ok",
		"id":      paymentId,
		"receipt": resp.Receipt,
		"env":     resp.Environment,
	})
}

// CancelApplePayment 取消 Apple 应用内支付
func (ctl *PaymentController) CancelApplePayment(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	if !ctl.applepay.Enabled() {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "Apple 应用内支付功能尚未开启"), http.StatusBadRequest)
	}

	paymentId := webCtx.PathVar("id")
	reason := webCtx.Input("reason")

	log.WithFields(log.Fields{"payment_id": paymentId, "reason": reason}).Info("cancel apple payment")

	if err := ctl.payRepo.CancelApplePayment(ctx, user.ID, paymentId, reason); err != nil {
		log.WithFields(log.Fields{
			"err":        err.Error(),
			"payment_id": paymentId,
			"user_id":    user.ID,
		}).Error("cancel payment failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(map[string]interface{}{
		"status": "ok",
		"id":     paymentId,
	})
}

// StripeWebhook stripe webhook
func (ctl *PaymentController) StripeWebhook(ctx context.Context, webCtx web.Context) web.Response {
	payload := webCtx.Body()
	signature := webCtx.Header("Stripe-Signature")

	event, err := webhook.ConstructEventWithOptions(
		payload,
		signature,
		ctl.conf.Stripe.WebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
	)
	if err != nil {
		log.F(log.M{"payload": string(payload), "signature": signature}).Errorf("stripe verifying webhook signature error: %s", err)
		return webCtx.JSONError("error verifying webhook signature", http.StatusBadRequest)
	}

	log.With(event).Infof("stripe webhook event: %s", event.Type)

	switch event.Type {
	case "charge.succeeded":
		if err := ctl.handleStripeChargeSucceeded(ctx, event); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("handle stripe charge succeeded failed")
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}
	case "payment_intent.succeeded":
		if err := ctl.handleStripePaymentIntentSucceeded(ctx, event); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("handle stripe payment intent succeeded failed")
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}
	}

	return webCtx.JSON(web.M{})
}

// handleStripeChargeSucceeded 处理 Stripe 支付成功事件
func (ctl *PaymentController) handleStripeChargeSucceeded(ctx context.Context, event stripe.Event) error {
	metadata := event.Data.Object["metadata"].(map[string]any)
	paymentID := metadata["payment_id"].(string)
	userID, _ := strconv.Atoi(metadata["user_id"].(string))

	pay := repo.StripePayment{}
	if liveMode, ok := event.Data.Object["livemode"].(bool); ok {
		pay.Environment = ternary.If(liveMode, "Production", "Test")
	}

	if receiptURL, ok := event.Data.Object["receipt_url"].(string); ok {
		pay.ReceiptURL = receiptURL
	}

	if details, ok := event.Data.Object["payment_method_details"]; ok {
		pay.Extra = details
	}

	return ctl.payRepo.UpdateStripePayment(ctx, int64(userID), paymentID, pay)
}

// handleStripePaymentIntentSucceeded 处理 Stripe 支付成功事件
func (ctl *PaymentController) handleStripePaymentIntentSucceeded(ctx context.Context, event stripe.Event) error {
	metadata := event.Data.Object["metadata"].(map[string]any)
	paymentID := metadata["payment_id"].(string)
	userID, _ := strconv.Atoi(metadata["user_id"].(string))
	productID := metadata["product_id"].(string)

	pay := repo.StripePayment{}
	if liveMode, ok := event.Data.Object["livemode"].(bool); ok {
		pay.Environment = ternary.If(liveMode, "Production", "Test")
	}

	pay.Amount = int64(event.Data.Object["amount"].(float64))
	pay.AmountReceived = int64(event.Data.Object["amount_received"].(float64))

	if currency, ok := event.Data.Object["currency"].(string); ok {
		pay.Currency = currency
	}

	pay.Status = int64(repo.PaymentStatusSuccess)

	eventID, err := ctl.payRepo.CompleteStripePayment(ctx, int64(userID), paymentID, pay)
	if err != nil {
		// 如果已经处理过了，直接返回成功
		if errors.Is(err, repo.ErrPaymentHasBeenProcessed) {
			return nil
		}

		log.WithFields(log.Fields{
			"err":   err.Error(),
			"event": event,
		}).Error("complete payment failed")

		return err
	}

	if eventID > 0 {
		payload := queue.PaymentPayload{
			UserID:    int64(userID),
			ProductID: productID,
			PaymentID: paymentID,
			Note:      coins.GetProduct(productID).Name,
			Source:    "stripe-purchase",
			Env:       ternary.If(event.Data.Object["livemode"].(bool), "Production", "Test"),
			CreatedAt: time.Now(),
			EventID:   eventID,
		}

		if _, err := ctl.queue.Enqueue(&payload, queue.NewPaymentTask); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("enqueue payment task failed")
		}
	}

	return nil
}

// CreateStripePayment 发起 Stripe 支付
func (ctl *PaymentController) CreateStripePayment(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {

	if !ctl.conf.Stripe.Enabled {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "Stripe 支付功能尚未开启"), http.StatusBadRequest)
	}

	productId := webCtx.Input("product_id")
	if productId == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	source := webCtx.Input("source")
	if source == "" {
		source = "app"
	}

	if !array.In(source, []string{"app", "web", "pc"}) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	product := coins.GetProduct(productId)
	if product == nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	if !array.In("stripe", product.GetSupportMethods()) {
		log.F(log.M{"user_id": user.ID}).Errorf("product %s not support stripe", productId)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	customerParams := &stripe.CustomerParams{}
	if user.Phone != "" {
		customerParams.Phone = stripe.String(user.Phone)
	}
	if user.Email != "" {
		customerParams.Email = stripe.String(user.Email)
	}
	if user.Name != "" {
		customerParams.Name = stripe.String(user.Name)
	}

	c, err := customer.New(customerParams)
	if err != nil {
		log.WithFields(log.Fields{"product_id": productId, "source": source}).Error("create stripe c failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	ek, err := ephemeralkey.New(&stripe.EphemeralKeyParams{
		Customer:      stripe.String(c.ID),
		StripeVersion: stripe.String("2023-10-16"),
	})
	if err != nil {
		log.WithFields(log.Fields{"product_id": productId, "source": source}).Error("create stripe ephemeral key failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	paymentIntentParams := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(product.GetRetailPriceUSD()),
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Customer: stripe.String(c.ID),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}

	paymentID := misc.PaymentID(user.ID)
	paymentIntentParams.AddMetadata("payment_id", paymentID)
	paymentIntentParams.AddMetadata("user_id", strconv.Itoa(int(user.ID)))
	paymentIntentParams.AddMetadata("product_id", productId)

	pi, err := paymentintent.New(paymentIntentParams)
	if err != nil {
		log.WithFields(log.Fields{"product_id": productId, "source": source}).Error("create stripe payment intent failed: %s", err)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	// 创建支付记录
	payment := repo.StripePayment{
		CustomerID:    c.ID,
		PaymentIntent: pi.ClientSecret,
		ProductID:     productId,
	}
	if _, err := ctl.payRepo.CreateStripePayment(ctx, user.ID, paymentID, source, payment); err != nil {
		log.WithFields(log.Fields{
			"err":        err.Error(),
			"product_id": productId,
			"user_id":    user.ID,
			"source":     source,
		}).Error("create stripe payment failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	proxyURL := ""
	if source == "pc" && ctl.conf.BaseURL != "" {
		proxyParams := url.Values{}
		proxyParams.Set("id", paymentID)
		proxyParams.Set("intent", pi.ClientSecret)
		proxyParams.Set("price", fmt.Sprintf("$%s", strconv.Itoa(int(product.GetRetailPriceUSD()/100))))
		proxyParams.Set("key", ctl.conf.Stripe.PublishableKey)
		proxyParams.Set("finish_action", "close")

		proxyURL = fmt.Sprintf("%s#/payment/proxy?%s", ctl.conf.BaseURL, proxyParams.Encode())
	}

	return webCtx.JSON(web.M{
		"payment_intent":  pi.ClientSecret,
		"ephemeral_key":   ek.Secret,
		"customer":        c.ID,
		"publishable_key": ctl.conf.Stripe.PublishableKey,
		"payment_id":      paymentID,
		"proxy_url":       proxyURL,
	})
}

// CreateWechatPayment create wechat payment
// @summary create wechat payment
// @tags payment
// @accept json
// @produce json
// @param product_id query string true "product id"
// @param source query string false "source" Enums(app,web,pc)
// @success 200 {object} WechatPayCreateResponse
// @router /v1/payment/wechatpay [post]
func (ctl *PaymentController) CreateWechatPayment(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	if !ctl.conf.WeChatPayEnabled {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "微信支付功能尚未开启"), http.StatusBadRequest)
	}

	productId := webCtx.Input("product_id")
	if productId == "" {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	source := webCtx.Input("source")
	if source == "" {
		source = "app"
	}

	if !array.In(source, []string{"app", "web", "pc"}) {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	product := coins.GetProduct(productId)
	if product == nil {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	if !array.In("wechat_pay", product.GetSupportMethods()) {
		log.F(log.M{"user_id": user.ID}).Errorf("product %s not support wechat-pay", productId)
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
	}

	paymentID, err := ctl.payRepo.CreateWechatPayment(ctx, user.ID, productId, source)
	if err != nil {
		log.WithFields(log.Fields{
			"err":        err.Error(),
			"product_id": productId,
			"user_id":    user.ID,
			"source":     source,
		}).Error("create payment failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	ret := WechatPayCreateResponse{
		PaymentID: paymentID,
		Sandbox:   false,
	}

	req := wechatpay.PrepayRequest{
		OutTradeNo:  paymentID,
		Description: product.Name,
		NotifyURL:   ctl.conf.WeChatPayNotifyURL,
		Amount:      product.RetailPrice,
	}

	switch source {
	case "pc", "web":
		resp, err := ctl.wechatpay.NativePrepay(ctx, req)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("create wechat payment failed")
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}
		ret.CodeURL = resp.CodeURL
	case "app":
		resp, err := ctl.wechatpay.AppPrepay(ctx, req)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("create wechat payment failed")
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}

		ret.PrepayID = resp.PrepayID
		ret.APPID = ctl.conf.WeChatAppID
		ret.Package = "Sign=WXPay"
		ret.PartnerID = ctl.conf.WeChatPayMchID
		ret.Noncestr = misc.ShortUUID()
		ret.Timestamp = strconv.FormatInt(time.Now().Unix(), 10)
		sign, err := ctl.wechatpay.SignAppPay(ret.APPID, ret.Timestamp, ret.Noncestr, ret.PrepayID)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("sign wechat app pay failed")
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
		}
		ret.Sign = sign
	}

	return webCtx.JSON(ret)
}

type WechatPayCreateResponse struct {
	PaymentID string `json:"payment_id"`
	Sandbox   bool   `json:"sandbox"`
	// CodeURL Web、PC 支付专用，二维码地址
	CodeURL string `json:"code_url,omitempty"`
	// PrepayID APP 支付专用，微信返回的支付交易会话ID，该值有效期为2小时。
	PrepayID string `json:"prepay_id,omitempty"`
	// Package APP 支付专用，固定值Sign=WXPay
	Package string `json:"package,omitempty"`
	// PartnerID APP 支付专用，商户号mchid对应的值
	PartnerID string `json:"partner_id,omitempty"`
	// APPID APP 支付专用，移动应用AppID
	APPID string `json:"app_id,omitempty"`
	// Noncestr APP 支付专用，随机字符串，不长于32位。推荐随机数生成算法
	Noncestr string `json:"noncestr,omitempty"`
	// Timestamp APP 支付专用，时间戳 秒级
	Timestamp string `json:"timestamp,omitempty"`
	// Sign APP 支付专用，签名，使用字段AppID、timeStamp、nonceStr、prepayid计算得出的签名值 注意：取值RSA格式
	Sign string `json:"sign,omitempty"`
}

// WechatPayNotify Wechat Pay result notification
// @summary Wechat Pay result notification
// @tags payment
// @accept json
// @produce json
// @router /v1/payment/callback/wechat-pay/notify [post]
func (ctl *PaymentController) WechatPayNotify(ctx context.Context, webCtx web.Context) web.Response {
	log.WithFields(log.Fields{
		"body": string(webCtx.Body()),
	}).Info("wechat pay callback")

	if !ctl.conf.WeChatPayEnabled {
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, "微信支付功能尚未开启"), http.StatusBadRequest)
	}

	mchPrivateKey, err := utils.LoadPrivateKeyWithPath(ctl.conf.WeChatPayCertPrivateKeyPath)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("load wechat pay cert private key failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if err := downloader.MgrInstance().RegisterDownloaderWithPrivateKey(
		ctx,
		mchPrivateKey,
		ctl.conf.WeChatPayCertSerialNumber,
		ctl.conf.WeChatPayMchID,
		ctl.conf.WeChatPayAPIv3Key,
	); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("register wechat pay downloader failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	certVisitor := downloader.MgrInstance().GetCertificateVisitor(ctl.conf.WeChatPayMchID)
	handler, err := notify.NewRSANotifyHandler(ctl.conf.WeChatPayAPIv3Key, verifiers.NewSHA256WithRSAVerifier(certVisitor))
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("create wechat pay notify handler failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	transaction := new(payments.Transaction)
	notifyReq, err := handler.ParseNotifyRequest(ctx, webCtx.Request().Raw(), transaction)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("parse wechat pay notify request failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	log.F(log.M{
		"transaction":   transaction,
		"decoded":       notifyReq.Resource.Plaintext,
		"event_type":    notifyReq.EventType,
		"resource_type": notifyReq.ResourceType,
		"summary":       notifyReq.Summary,
	}).Debugf("wechat pay notify request: %v", notifyReq)

	if notifyReq.EventType != "TRANSACTION.SUCCESS" {
		return webCtx.JSON(web.M{})
	}

	his, err := ctl.payRepo.GetPaymentHistory(ctx, *transaction.OutTradeNo)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("get payment history failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	wechatPayHis, err := ctl.payRepo.GetWechatHistory(ctx, his.PaymentId)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("get wechat pay history failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	purchaseAt, _ := time.Parse(time.RFC3339, *transaction.SuccessTime)
	eventID, err := ctl.payRepo.CompleteWechatPayment(ctx, his.UserId, his.PaymentId, repo.WechatPayment{
		ProductID:   wechatPayHis.ProductId,
		Extra:       notifyReq.Resource.Plaintext,
		Amount:      *transaction.Amount.Total,
		Environment: "Production",
		PurchaseAt:  purchaseAt,
		Status:      repo.PaymentStatusSuccess,
		Note:        notifyReq.Summary,
	})
	if err != nil {
		// 如果已经处理过了，直接返回成功
		if errors.Is(err, repo.ErrPaymentHasBeenProcessed) {
			return webCtx.JSON(web.M{})
		}

		log.WithFields(log.Fields{
			"err":         err.Error(),
			"transaction": transaction,
		}).Error("complete payment failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	if eventID > 0 {
		payload := queue.PaymentPayload{
			UserID:    his.UserId,
			ProductID: wechatPayHis.ProductId,
			PaymentID: his.PaymentId,
			Note:      coins.GetProduct(wechatPayHis.ProductId).Name,
			Source:    "wechat-purchase",
			Env:       "Production",
			CreatedAt: time.Now(),
			EventID:   eventID,
		}

		if _, err := ctl.queue.Enqueue(&payload, queue.NewPaymentTask); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("enqueue payment task failed")
		}
	}

	return webCtx.JSON(web.M{})
}
