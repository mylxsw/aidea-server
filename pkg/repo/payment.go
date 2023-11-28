package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/mylxsw/aidea-server/internal/coins"
	model2 "github.com/mylxsw/aidea-server/pkg/repo/model"
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

func (repo *PaymentRepo) GetPaymentHistory(ctx context.Context, userID int64, paymentID string) (model2.PaymentHistory, error) {
	q := query.Builder().
		Where(model2.FieldPaymentHistoryPaymentId, paymentID).
		Where(model2.FieldPaymentHistoryUserId, userID)

	pay, err := model2.NewPaymentHistoryModel(repo.db).First(ctx, q)
	if err != nil {
		if err == query.ErrNoResult {
			return model2.PaymentHistory{}, ErrNotFound
		}
		return model2.PaymentHistory{}, err
	}

	return pay.ToPaymentHistory(), nil
}

func (repo *PaymentRepo) CreateAliPayment(ctx context.Context, userID int64, productID string, source string) (string, error) {
	paymentID, err := uuid.GenerateUUID()
	if err != nil {
		return "", fmt.Errorf("generate payment id failed: %w", err)
	}

	product := coins.GetProduct(productID)
	if product == nil {
		return "", fmt.Errorf("product %s not found", productID)
	}

	paymentID = fmt.Sprintf("%d-%s", userID, paymentID)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model2.NewPaymentHistoryModel(tx).Create(ctx, query.KV{
			model2.FieldPaymentHistoryPaymentId:   paymentID,
			model2.FieldPaymentHistoryUserId:      userID,
			model2.FieldPaymentHistorySource:      "alipay-" + source,
			model2.FieldPaymentHistoryStatus:      PaymentStatusWaiting,
			model2.FieldPaymentHistoryRetailPrice: product.RetailPrice,
			model2.FieldPaymentHistoryQuantity:    product.Quota,
			model2.FieldPaymentHistoryValidUntil:  product.ExpiredAt(),
		}); err != nil {
			return fmt.Errorf("create payment history failed: %w", err)
		}

		if _, err := model2.NewAlipayHistoryModel(tx).Create(ctx, query.KV{
			model2.FieldAlipayHistoryPaymentId: paymentID,
			model2.FieldAlipayHistoryProductId: productID,
			model2.FieldAlipayHistoryUserId:    userID,
			model2.FieldAlipayHistoryStatus:    PaymentStatusWaiting,
		}); err != nil {
			return fmt.Errorf("create alipay history failed: %w", err)
		}

		return nil
	})

	return paymentID, err
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
		Where(model2.FieldPaymentHistoryPaymentId, paymentID).
		Where(model2.FieldPaymentHistoryUserId, userId)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		his, err := model2.NewPaymentHistoryModel(tx).First(ctx, q)
		if err != nil {
			return fmt.Errorf("get payment history failed: %w", err)
		}

		if his.Status.ValueOrZero() != PaymentStatusWaiting {
			return ErrPaymentHasBeenProcessed
		}

		if _, err := model2.NewPaymentHistoryModel(tx).Update(ctx, q, model2.PaymentHistoryN{
			Status:      null.IntFrom(pay.Status),
			Environment: null.StringFrom(pay.Environment),
			PurchaseAt:  null.TimeFrom(pay.PurchaseAt),
		}); err != nil {
			return fmt.Errorf("update payment history failed: %w", err)
		}

		if _, err := model2.NewAlipayHistoryModel(tx).Update(ctx, q, model2.AlipayHistoryN{
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
			if eventID, err = model2.NewEventsModel(tx).Save(ctx, model2.EventsN{
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

func (repo *PaymentRepo) GetAlipayHistory(ctx context.Context, paymentID string) (*model2.AlipayHistory, error) {
	his, err := model2.NewAlipayHistoryModel(repo.db).First(ctx, query.Builder().Where(model2.FieldAlipayHistoryPaymentId, paymentID))
	if err != nil {
		if err == query.ErrNoResult {
			return nil, ErrNotFound
		}

		return nil, err
	}

	ret := his.ToAlipayHistory()
	return &ret, nil
}

func (repo *PaymentRepo) CreateApplePayment(ctx context.Context, userID int64, productID string) (string, error) {
	paymentID, err := uuid.GenerateUUID()
	if err != nil {
		return "", fmt.Errorf("generate payment id failed: %w", err)
	}

	product := coins.GetProduct(productID)
	if product == nil {
		return "", fmt.Errorf("product %s not found", productID)
	}

	paymentID = fmt.Sprintf("%d-%s", userID, paymentID)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model2.NewPaymentHistoryModel(tx).Create(ctx, query.KV{
			model2.FieldPaymentHistoryPaymentId:   paymentID,
			model2.FieldPaymentHistoryUserId:      userID,
			model2.FieldPaymentHistorySource:      "apple",
			model2.FieldPaymentHistoryStatus:      PaymentStatusWaiting,
			model2.FieldPaymentHistoryRetailPrice: product.RetailPrice,
			model2.FieldPaymentHistoryQuantity:    product.Quota,
			model2.FieldPaymentHistoryValidUntil:  product.ExpiredAt(),
		}); err != nil {
			return fmt.Errorf("create payment history failed: %w", err)
		}

		if _, err := model2.NewApplePayHistoryModel(tx).Create(ctx, query.KV{
			model2.FieldApplePayHistoryPaymentId: paymentID,
			model2.FieldApplePayHistoryProductId: productID,
			model2.FieldApplePayHistoryUserId:    userID,
			model2.FieldApplePayHistoryStatus:    PaymentStatusWaiting,
		}); err != nil {
			return fmt.Errorf("create apple pay history failed: %w", err)
		}

		return nil
	})

	return paymentID, err
}

func (repo *PaymentRepo) UpdateApplePayment(ctx context.Context, userId int64, paymentID string, source, serverVerifyData string) error {
	q := query.Builder().Where(model2.FieldApplePayHistoryPaymentId, paymentID).
		Where(model2.FieldApplePayHistoryUserId, userId).
		Where(model2.FieldApplePayHistoryStatus, PaymentStatusWaiting)
	if _, err := model2.NewApplePayHistoryModel(repo.db).Update(
		ctx,
		q,
		model2.ApplePayHistoryN{
			Source:           null.StringFrom(source),
			ServerVerifyData: null.StringFrom(serverVerifyData),
		},
		model2.FieldApplePayHistorySource,
		model2.FieldApplePayHistoryServerVerifyData,
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
		Where(model2.FieldPaymentHistoryPaymentId, paymentID).
		Where(model2.FieldPaymentHistoryUserId, userId)
	err = eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model2.NewPaymentHistoryModel(tx).Update(ctx, q, model2.PaymentHistoryN{
			Status:      null.IntFrom(applePayment.Status),
			Environment: null.StringFrom(applePayment.Environment),
			PurchaseAt:  null.TimeFrom(applePayment.PurchaseAt),
		}); err != nil {
			return fmt.Errorf("update payment history failed: %w", err)
		}

		if _, err := model2.NewApplePayHistoryModel(tx).Update(ctx, q, model2.ApplePayHistoryN{
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
			if eventID, err = model2.NewEventsModel(tx).Save(ctx, model2.EventsN{
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
		Where(model2.FieldPaymentHistoryPaymentId, paymentID).
		Where(model2.FieldPaymentHistoryUserId, userId)

	return eloquent.Transaction(repo.db, func(tx query.Database) error {
		if _, err := model2.NewPaymentHistoryModel(tx).Update(ctx, q, model2.PaymentHistoryN{
			Status: null.IntFrom(PaymentStatusCanceled),
		}); err != nil {
			return fmt.Errorf("update payment history failed: %w", err)
		}

		if _, err := model2.NewApplePayHistoryModel(tx).Update(ctx, q, model2.ApplePayHistoryN{
			Status: null.IntFrom(PaymentStatusCanceled),
			Note:   null.StringFrom(reason),
		}); err != nil {
			return fmt.Errorf("update apple pay history failed: %w", err)
		}

		return nil
	})
}
