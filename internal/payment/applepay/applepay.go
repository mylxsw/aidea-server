package applepay

import (
	"context"
	"errors"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo"
	"strconv"

	"github.com/awa/go-iap/appstore"
	"github.com/mylxsw/asteria/log"
)

type ApplePay interface {
	Enabled() bool
	VerifyPayment(ctx context.Context, purchaseId string, serverVerifyData string) (*repo.ApplePayment, *appstore.IAPResponse, *appstore.InApp, error)
}

type ApplePayImpl struct{}

func NewApplePay() *ApplePayImpl {
	return &ApplePayImpl{}
}

func (pay *ApplePayImpl) Enabled() bool {
	return true
}

func (pay *ApplePayImpl) VerifyPayment(ctx context.Context, purchaseId string, serverVerifyData string) (*repo.ApplePayment, *appstore.IAPResponse, *appstore.InApp, error) {
	client := appstore.New()
	req := appstore.IAPRequest{
		ReceiptData: serverVerifyData,
	}
	var resp appstore.IAPResponse
	err := client.Verify(ctx, req, &resp)
	if err != nil {
		log.WithFields(log.Fields{"err": err.Error()}).Error("verify payment failed")
		return nil, nil, nil, errors.New("支付验证失败")
	}

	// 从 resp.Receipt.InApp 中取出支付时间最晚的一项，这是最新的一次购买
	var inApp appstore.InApp
	for _, item := range resp.Receipt.InApp {
		itemPurchaseTs, _ := strconv.Atoi(item.PurchaseDateMS)
		inAppPurchaseTs, _ := strconv.Atoi(inApp.PurchaseDateMS)

		if itemPurchaseTs > inAppPurchaseTs {
			inApp = item
		}
	}

	purchaseAt, err := misc.ParseAppleDateTime(inApp.PurchaseDate.PurchaseDate)
	if err != nil {
		log.WithFields(log.Fields{
			"err":           err.Error(),
			"purchase_date": inApp.PurchaseDate.PurchaseDate,
		}).Error("parse apple purchase date failed")
	}

	applePayment := repo.ApplePayment{
		PurchaseID:    purchaseId,
		TransactionID: inApp.TransactionID,
		Environment:   string(resp.Environment),
		PurchaseAt:    purchaseAt,
	}

	return &applePayment, &resp, &inApp, nil
}

type ApplePayFake struct{}

func (pay *ApplePayFake) Enabled() bool {
	return false
}

func (pay *ApplePayFake) VerifyPayment(ctx context.Context, purchaseId string, serverVerifyData string) (*repo.ApplePayment, *appstore.IAPResponse, *appstore.InApp, error) {
	panic("implement me")
}
