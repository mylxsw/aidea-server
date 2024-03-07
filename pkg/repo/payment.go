package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mylxsw/aidea-server/internal/coins"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/mylxsw/eloquent"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/go-utils/must"
	"github.com/mylxsw/go-utils/ternary"
	"gopkg.in/guregu/null.v3"
)

const (
	PaymentStatusWaiting  = 0
	PaymentStatusSuccess  = 1
	PaymentStatusFailed   = 2
	PaymentStatusCanceled = 3
)

var (
	ErrPaymentHasBeenProcessed = fmt.Errorf("payment has been processed")
)

type PaymentRepo struct {
	db *sql.DB
}

func NewPaymentRepo(db *sql.DB) *PaymentRepo {
	return &PaymentRepo{db: db}
}

func (repo *PaymentRepo) GetPaymentHistory(ctx context.Context, userID int64, paymentID string) (model.PaymentHistory, error) {
	q := query.Builder().
		Where(model.FieldPaymentHistoryPaymentId, paymentID).
		Where(model.FieldPaymentHistoryUserId, userID)

	pay, err := model.NewPaymentHistoryModel(repo.db).First(ctx, q)
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return model.PaymentHistory{}, ErrNotFound
		}
		return model.PaymentHistory{}, err
	}

	return pay.ToPaymentHistory(), nil
}

func (repo *PaymentRepo) CreateAliPayment(ctx context.Context, userID int64, productID string, source string) (string, error) {
	paymentID, err := repo.CreatePaymentID(userID)
	if err != nil {
		return "", err
	}

	product := coins.GetProduct(productID)
	if product == nil {
		return "", fmt.Errorf("product %s not found", productID)
	}

	paymentID = fmt.Sprintf("%d-%s", userID, paymentID)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model.NewPaymentHistoryModel(tx).Create(ctx, query.KV{
			model.FieldPaymentHistoryPaymentId:   paymentID,
			model.FieldPaymentHistoryUserId:      userID,
			model.FieldPaymentHistorySource:      "alipay-" + source,
			model.FieldPaymentHistoryStatus:      PaymentStatusWaiting,
			model.FieldPaymentHistoryRetailPrice: product.RetailPrice,
			model.FieldPaymentHistoryQuantity:    product.Quota,
			model.FieldPaymentHistoryValidUntil:  product.ExpiredAt(),
		}); err != nil {
			return fmt.Errorf("create payment history failed: %w", err)
		}

		if _, err := model.NewAlipayHistoryModel(tx).Create(ctx, query.KV{
			model.FieldAlipayHistoryPaymentId: paymentID,
			model.FieldAlipayHistoryProductId: productID,
			model.FieldAlipayHistoryUserId:    userID,
			model.FieldAlipayHistoryStatus:    PaymentStatusWaiting,
		}); err != nil {
			return fmt.Errorf("create alipay history failed: %w", err)
		}

		return nil
	})

	return paymentID, err
}

type StripePayment struct {
	CustomerID string `json:"customer_id,omitempty"`
	ProductID  string `json:"product_id,omitempty"`

	Amount         int64  `json:"amount,omitempty"`
	AmountReceived int64  `json:"amount_received,omitempty"`
	Currency       string `json:"currency,omitempty"`

	ReceiptURL    string `json:"receipt_url,omitempty"`
	Environment   string `json:"environment,omitempty"`
	PaymentIntent string `json:"payment_intent,omitempty"`
	Status        int64  `json:"status,omitempty"`
	Note          string `json:"note,omitempty"`
	Extra         any    `json:"extra,omitempty"`
}

// CreatePaymentID 生成支付ID
func (repo *PaymentRepo) CreatePaymentID(userID int64) (string, error) {
	paymentID, err := uuid.GenerateUUID()
	if err != nil {
		return "", fmt.Errorf("generate payment id failed: %w", err)
	}

	return fmt.Sprintf("%d-%s", userID, paymentID), nil
}

func (repo *PaymentRepo) CreateStripePayment(ctx context.Context, userID int64, paymentID string, source string, pay StripePayment) (string, error) {
	product := coins.GetProduct(pay.ProductID)
	if product == nil {
		return "", fmt.Errorf("product %s not found", pay.ProductID)
	}

	err := eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model.NewPaymentHistoryModel(tx).Create(ctx, query.KV{
			model.FieldPaymentHistoryPaymentId:   paymentID,
			model.FieldPaymentHistoryUserId:      userID,
			model.FieldPaymentHistorySource:      "stripe-" + source,
			model.FieldPaymentHistoryStatus:      PaymentStatusWaiting,
			model.FieldPaymentHistoryRetailPrice: product.RetailPrice,
			model.FieldPaymentHistoryQuantity:    product.Quota,
			model.FieldPaymentHistoryValidUntil:  product.ExpiredAt(),
		}); err != nil {
			return fmt.Errorf("create payment history failed: %w", err)
		}

		data := query.KV{
			model.FieldStripeHistoryPaymentId: paymentID,
			model.FieldStripeHistoryProductId: pay.ProductID,
			model.FieldStripeHistoryUserId:    userID,
			model.FieldStripeHistoryStatus:    PaymentStatusWaiting,
		}

		if pay.CustomerID != "" {
			data[model.FieldStripeHistoryCustomerId] = pay.CustomerID
		}

		if pay.PaymentIntent != "" {
			data[model.FieldStripeHistoryPaymentIntent] = pay.PaymentIntent
		}

		if _, err := model.NewStripeHistoryModel(tx).Create(ctx, data); err != nil {
			return fmt.Errorf("create stripe history failed: %w", err)
		}

		return nil
	})

	return paymentID, err
}

func (repo *PaymentRepo) UpdateStripePayment(ctx context.Context, userId int64, paymentID string, pay StripePayment) error {
	q := query.Builder().
		Where(model.FieldStripeHistoryPaymentId, paymentID).
		Where(model.FieldStripeHistoryUserId, userId)
	his, err := model.NewStripeHistoryModel(repo.db).First(ctx, q)
	if err != nil {
		return fmt.Errorf("get stripe history failed: %w", err)
	}

	saveFields := make([]string, 0)

	if pay.Environment != "" {
		his.Environment = null.StringFrom(pay.Environment)
		saveFields = append(saveFields, model.FieldStripeHistoryEnvironment)
	}
	if pay.PaymentIntent != "" {
		his.PaymentIntent = null.StringFrom(pay.PaymentIntent)
		saveFields = append(saveFields, model.FieldStripeHistoryPaymentIntent)
	}
	if pay.ReceiptURL != "" {
		his.ReceiptUrl = null.StringFrom(pay.ReceiptURL)
		saveFields = append(saveFields, model.FieldStripeHistoryReceiptUrl)
	}

	if pay.Extra != nil {
		data, _ := json.Marshal(pay.Extra)
		his.Extra = null.StringFrom(string(data))
		saveFields = append(saveFields, model.FieldStripeHistoryExtra)
	}

	if len(saveFields) == 0 {
		return nil
	}

	return his.Save(ctx, saveFields...)
}

func (repo *PaymentRepo) CompleteStripePayment(ctx context.Context, userId int64, paymentID string, pay StripePayment) (eventID int64, err error) {
	q := query.Builder().
		Where(model.FieldPaymentHistoryPaymentId, paymentID).
		Where(model.FieldPaymentHistoryUserId, userId)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		his, err := model.NewPaymentHistoryModel(tx).First(ctx, q)
		if err != nil {
			return fmt.Errorf("get payment history failed: %w", err)
		}

		if his.Status.ValueOrZero() != PaymentStatusWaiting {
			return ErrPaymentHasBeenProcessed
		}

		purchaseTime := time.Now()

		if _, err := model.NewPaymentHistoryModel(tx).Update(ctx, q, model.PaymentHistoryN{
			Status:      null.IntFrom(pay.Status),
			Environment: null.StringFrom(pay.Environment),
			PurchaseAt:  null.TimeFrom(purchaseTime),
		}); err != nil {
			return fmt.Errorf("update payment history failed: %w", err)
		}

		stripeHistory := model.StripeHistoryN{
			Status:     null.IntFrom(pay.Status),
			PurchaseAt: null.TimeFrom(purchaseTime),
			Note:       ternary.If(pay.Note != "", null.StringFrom(pay.Note), null.NewString("", false)),
		}

		if pay.Amount > 0 {
			stripeHistory.Amount = null.IntFrom(pay.Amount)
		}
		if pay.AmountReceived > 0 {
			stripeHistory.AmountReceived = null.IntFrom(pay.AmountReceived)
		}
		if pay.Currency != "" {
			stripeHistory.Currency = null.StringFrom(pay.Currency)
		}
		if pay.Environment != "" {
			stripeHistory.Environment = null.StringFrom(pay.Environment)
		}

		if _, err := model.NewStripeHistoryModel(tx).Update(ctx, q, stripeHistory); err != nil {
			return fmt.Errorf("update apple pay history failed: %w", err)
		}

		if pay.Status == PaymentStatusSuccess {
			if eventID, err = model.NewEventsModel(tx).Save(ctx, model.EventsN{
				EventType: null.StringFrom(EventTypePaymentCompleted),
				Payload: null.StringFrom(string(must.Must(json.Marshal(PaymentCompletedEvent{
					UserID:    userId,
					ProductID: pay.ProductID,
					PaymentID: paymentID,
				})))),
				Status: null.StringFrom(EventStatusWaiting),
			}); err != nil {
				return fmt.Errorf("create event failed: %w", err)
			}
		}

		return nil
	})

	return eventID, err
}

type AlipayPayment struct {
	ProductID      string    `json:"product_id"`
	BuyerID        string    `json:"buyer_id"`
	InvoiceAmount  int64     `json:"invoice_amount"`
	ReceiptAmount  int64     `json:"receipt_amount"`
	BuyerPayAmount int64     `json:"buyer_pay_amount"`
	TotalAmount    int64     `json:"total_amount"`
	PointAmount    int64     `json:"point_amount"`
	TradeNo        string    `json:"trade_no"`
	BuyerLogonID   string    `json:"buyer_logon_id"`
	PurchaseAt     time.Time `json:"purchase_at"`
	Status         int64     `json:"status"`
	Environment    string    `json:"environment"`
	Note           string    `json:"note"`
}

func (repo *PaymentRepo) CompleteAliPayment(ctx context.Context, userId int64, paymentID string, pay AlipayPayment) (eventID int64, err error) {
	q := query.Builder().
		Where(model.FieldPaymentHistoryPaymentId, paymentID).
		Where(model.FieldPaymentHistoryUserId, userId)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		his, err := model.NewPaymentHistoryModel(tx).First(ctx, q)
		if err != nil {
			return fmt.Errorf("get payment history failed: %w", err)
		}

		if his.Status.ValueOrZero() != PaymentStatusWaiting {
			return ErrPaymentHasBeenProcessed
		}

		if _, err := model.NewPaymentHistoryModel(tx).Update(ctx, q, model.PaymentHistoryN{
			Status:      null.IntFrom(pay.Status),
			Environment: null.StringFrom(pay.Environment),
			PurchaseAt:  null.TimeFrom(pay.PurchaseAt),
		}); err != nil {
			return fmt.Errorf("update payment history failed: %w", err)
		}

		if _, err := model.NewAlipayHistoryModel(tx).Update(ctx, q, model.AlipayHistoryN{
			BuyerId:        null.StringFrom(pay.BuyerID),
			InvoiceAmount:  null.IntFrom(pay.InvoiceAmount),
			ReceiptAmount:  null.IntFrom(pay.ReceiptAmount),
			BuyerPayAmount: null.IntFrom(pay.BuyerPayAmount),
			TotalAmount:    null.IntFrom(pay.TotalAmount),
			PointAmount:    null.IntFrom(pay.PointAmount),
			TradeNo:        null.StringFrom(pay.TradeNo),
			BuyerLogonId:   null.StringFrom(pay.BuyerLogonID),
			Status:         null.IntFrom(pay.Status),
			PurchaseAt:     null.TimeFrom(pay.PurchaseAt),
			Note:           ternary.If(pay.Note != "", null.StringFrom(pay.Note), null.NewString("", false)),
		}); err != nil {
			return fmt.Errorf("update apple pay history failed: %w", err)
		}

		if pay.Status == PaymentStatusSuccess {
			if eventID, err = model.NewEventsModel(tx).Save(ctx, model.EventsN{
				EventType: null.StringFrom(EventTypePaymentCompleted),
				Payload: null.StringFrom(string(must.Must(json.Marshal(PaymentCompletedEvent{
					UserID:    userId,
					ProductID: pay.ProductID,
					PaymentID: paymentID,
				})))),
				Status: null.StringFrom(EventStatusWaiting),
			}); err != nil {
				return fmt.Errorf("create event failed: %w", err)
			}
		}

		return nil
	})

	return eventID, err
}

func (repo *PaymentRepo) GetAlipayHistory(ctx context.Context, paymentID string) (*model.AlipayHistory, error) {
	his, err := model.NewAlipayHistoryModel(repo.db).First(ctx, query.Builder().Where(model.FieldAlipayHistoryPaymentId, paymentID))
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	ret := his.ToAlipayHistory()
	return &ret, nil
}

func (repo *PaymentRepo) CreateApplePayment(ctx context.Context, userID int64, productID string) (string, error) {
	paymentID, err := repo.CreatePaymentID(userID)
	if err != nil {
		return "", err
	}

	product := coins.GetProduct(productID)
	if product == nil {
		return "", fmt.Errorf("product %s not found", productID)
	}

	paymentID = fmt.Sprintf("%d-%s", userID, paymentID)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model.NewPaymentHistoryModel(tx).Create(ctx, query.KV{
			model.FieldPaymentHistoryPaymentId:   paymentID,
			model.FieldPaymentHistoryUserId:      userID,
			model.FieldPaymentHistorySource:      "apple",
			model.FieldPaymentHistoryStatus:      PaymentStatusWaiting,
			model.FieldPaymentHistoryRetailPrice: product.RetailPrice,
			model.FieldPaymentHistoryQuantity:    product.Quota,
			model.FieldPaymentHistoryValidUntil:  product.ExpiredAt(),
		}); err != nil {
			return fmt.Errorf("create payment history failed: %w", err)
		}

		if _, err := model.NewApplePayHistoryModel(tx).Create(ctx, query.KV{
			model.FieldApplePayHistoryPaymentId: paymentID,
			model.FieldApplePayHistoryProductId: productID,
			model.FieldApplePayHistoryUserId:    userID,
			model.FieldApplePayHistoryStatus:    PaymentStatusWaiting,
		}); err != nil {
			return fmt.Errorf("create apple pay history failed: %w", err)
		}

		return nil
	})

	return paymentID, err
}

func (repo *PaymentRepo) UpdateApplePayment(ctx context.Context, userId int64, paymentID string, source, serverVerifyData string) error {
	q := query.Builder().Where(model.FieldApplePayHistoryPaymentId, paymentID).
		Where(model.FieldApplePayHistoryUserId, userId).
		Where(model.FieldApplePayHistoryStatus, PaymentStatusWaiting)
	if _, err := model.NewApplePayHistoryModel(repo.db).Update(
		ctx,
		q,
		model.ApplePayHistoryN{
			Source:           null.StringFrom(source),
			ServerVerifyData: null.StringFrom(serverVerifyData),
		},
		model.FieldApplePayHistorySource,
		model.FieldApplePayHistoryServerVerifyData,
	); err != nil {
		return err
	}

	return nil
}

type ApplePayment struct {
	PurchaseID    string    `json:"purchase_id"`
	TransactionID string    `json:"transaction_id"`
	Environment   string    `json:"environment"`
	PurchaseAt    time.Time `json:"purchase_at"`
	Status        int64     `json:"status"`
}

func (repo *PaymentRepo) CompleteApplePayment(ctx context.Context, userId int64, paymentID string, applePayment *ApplePayment) (eventID int64, err error) {
	q := query.Builder().
		Where(model.FieldPaymentHistoryPaymentId, paymentID).
		Where(model.FieldPaymentHistoryUserId, userId)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model.NewPaymentHistoryModel(tx).Update(ctx, q, model.PaymentHistoryN{
			Status:      null.IntFrom(applePayment.Status),
			Environment: null.StringFrom(applePayment.Environment),
			PurchaseAt:  null.TimeFrom(applePayment.PurchaseAt),
		}); err != nil {
			return fmt.Errorf("update payment history failed: %w", err)
		}

		if _, err := model.NewApplePayHistoryModel(tx).Update(ctx, q, model.ApplePayHistoryN{
			Status:        null.IntFrom(applePayment.Status),
			Environment:   null.StringFrom(applePayment.Environment),
			PurchaseAt:    null.TimeFrom(applePayment.PurchaseAt),
			PurchaseId:    null.StringFrom(applePayment.PurchaseID),
			TransactionId: null.StringFrom(applePayment.TransactionID),
			Note: ternary.If(
				applePayment.Status == PaymentStatusFailed,
				null.StringFrom("验证失败，交易信息存在异常"),
				null.NewString("", false),
			),
		}); err != nil {
			return fmt.Errorf("update apple pay history failed: %w", err)
		}

		if applePayment.Status == PaymentStatusSuccess {
			if eventID, err = model.NewEventsModel(tx).Save(ctx, model.EventsN{
				EventType: null.StringFrom(EventTypePaymentCompleted),
				Payload: null.StringFrom(string(must.Must(json.Marshal(PaymentCompletedEvent{
					UserID:    userId,
					ProductID: applePayment.PurchaseID,
					PaymentID: paymentID,
				})))),
				Status: null.StringFrom(EventStatusWaiting),
			}); err != nil {
				return fmt.Errorf("create event failed: %w", err)
			}
		}

		return nil
	})

	return eventID, err
}

func (repo *PaymentRepo) CancelApplePayment(ctx context.Context, userId int64, paymentID string, reason string) error {
	q := query.Builder().
		Where(model.FieldPaymentHistoryPaymentId, paymentID).
		Where(model.FieldPaymentHistoryUserId, userId)

	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model.NewPaymentHistoryModel(tx).Update(ctx, q, model.PaymentHistoryN{
			Status: null.IntFrom(PaymentStatusCanceled),
		}); err != nil {
			return fmt.Errorf("update payment history failed: %w", err)
		}

		if _, err := model.NewApplePayHistoryModel(tx).Update(ctx, q, model.ApplePayHistoryN{
			Status: null.IntFrom(PaymentStatusCanceled),
			Note:   null.StringFrom(reason),
		}); err != nil {
			return fmt.Errorf("update apple pay history failed: %w", err)
		}

		return nil
	})
}
