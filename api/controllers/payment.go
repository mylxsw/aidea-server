package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mylxsw/aidea-server/internal/payment/applepay"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/aidea-server/api/controllers/common"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/internal/payment/alipay"
	"github.com/mylxsw/aidea-server/internal/queue"
	"github.com/mylxsw/aidea-server/internal/repo"
	"github.com/mylxsw/aidea-server/internal/youdao"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/glacier/web"
	"github.com/mylxsw/go-utils/array"
)

type PaymentController struct {
	translater youdao.Translater `autowire:"@"`
	queue      *queue.Queue      `autowire:"@"`
	payRepo    *repo.PaymentRepo `autowire:"@"`
	alipay     alipay.Alipay     `autowire:"@"`
	applepay   applepay.ApplePay `autowire:"@"`
}

func NewPaymentController(resolver infra.Resolver) web.Controller {
	ctl := PaymentController{}
	resolver.MustAutoWire(&ctl)

	return &ctl
}

func (p *PaymentController) Register(router web.Router) {
	router.Group("/payment", func(router web.Router) {
		// Apple 应用内支付
		router.Get("/apple/products", p.AppleProducts)
		router.Post("/apple", p.CreateApplePayment)
		router.Put("/apple/{id}", p.UpdateApplePayment)
		router.Delete("/apple/{id}", p.CancelApplePayment)
		router.Post("/apple/{id}/verify", p.VerifyApplePayment)

		// 支付宝支付
		router.Get("/alipay/products", p.AppleProducts)
		router.Group("/alipay", func(router web.Router) {
			router.Post("/", p.CreateAlipay)
			router.Post("/client-confirm", p.AlipayClientConfirm)
		})

		// 支付状态查询
		router.Group("/status", func(router web.Router) {
			router.Get("/{id}", p.QueryPaymentStatus)
		})

		// 支付结果回调通知
		router.Group("/callback", func(router web.Router) {
			router.Post("/alipay-notify", p.AlipayNotify)
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

	product := coins.GetAppleProduct(productId)
	if product == nil {
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

	payParams, err := ctl.alipay.TradePay(ctx, source, trade, true)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("create alipay payment failed")
		return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInternalError), http.StatusInternalServerError)
	}

	return webCtx.JSON(web.M{"params": payParams, "payment_id": paymentID})
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

	his, err := ctl.payRepo.GetPaymentHistory(ctx, user.ID, res.AlipayTradeAppPayResponse.OutTradeNo)
	if err != nil {
		if err == repo.ErrNotFound {
			return webCtx.JSONError(common.Text(webCtx, ctl.translater, common.ErrInvalidRequest), http.StatusBadRequest)
		}

		log.WithFields(log.Fields{"err": err}).Error("get payment history failed")
	}

	if his.Status != int(repo.PaymentStatusSuccess) {
		log.WithFields(log.Fields{
			"his":    his,
			"result": res,
		}).Errorf("客户端获取支付宝支付状态失败，支付状态不是成功状态")
	}

	return webCtx.JSON(web.M{
		"status": "ok",
	})
}

// QueryPaymentStatus 查询支付状态
func (ctl *PaymentController) QueryPaymentStatus(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	paymentId := webCtx.PathVar("id")
	history, err := ctl.payRepo.GetPaymentHistory(ctx, user.ID, paymentId)
	if err != nil {
		if err == repo.ErrNotFound {
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

	product := coins.GetAppleProduct(productId)

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
		Note:           note,
	}
	eventID, err := ctl.payRepo.CompleteAliPayment(ctx, int64(userId), paymentId, aliPay)
	if err != nil {
		// 如果已经处理过了，直接返回成功
		if err == repo.ErrPaymentHasBeenProcessed {
			return webCtx.Raw(func(w http.ResponseWriter) {
				w.Write([]byte("success"))
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
			Env:       "Production",
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
func (ctl *PaymentController) AppleProducts(ctx context.Context, webCtx web.Context, user *auth.User) web.Response {
	products := array.Map(coins.AppleProducts, func(product coins.AppleProduct, _ int) coins.AppleProduct {
		product.ExpirePolicyText = product.GetExpirePolicyText()
		if product.RetailPrice == 0 {
			product.RetailPrice = product.Quota
		}
		return product
	})

	return webCtx.JSON(web.M{
		"consume": products,
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

	if !coins.IsAppleProduct(productId) {
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

	product := coins.GetAppleProduct(receiptProductId)
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
