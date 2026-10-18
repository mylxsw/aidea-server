package externalpay

import (
	"context"
	"testing"
)

// Run against a disposable database with AIDEA_EXTERNALPAY_TEST_DSN set.
func TestExternalPaymentDatabaseFlow(t *testing.T) {
	db := externalPayTestDB(t)
	ctx := context.Background()
	var err error
	clientID, err := randomID("app_")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO external_pay_clients (client_id,public_key_pem) VALUES (?,?)`, clientID, "test-public-key"); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	paymentID, err := randomPaymentID()
	if err != nil {
		t.Fatal(err)
	}
	o := Order{PaymentID: paymentID, ClientID: clientID, MerchantOrderID: "business-1", Channel: "wechat_native", AmountFen: 1990, Currency: "CNY", Subject: "test item", CallbackURL: "https://example.com/notify"}
	if err := store.insertOrder(ctx, o); err != nil {
		t.Fatal(err)
	}
	if err := store.prepayReady(ctx, paymentID, "weixin://wxpay/example"); err != nil {
		t.Fatal(err)
	}
	tradeID, err := randomID("wx_")
	if err != nil {
		t.Fatal(err)
	}
	if created, err := store.merchantOrder(ctx, clientID, "business-1"); err != nil || created.Status != "pending" || created.CallbackURL != o.CallbackURL {
		t.Fatalf("order not stored correctly: %+v, %v", created, err)
	}
	if changed, err := store.markPaid(ctx, paymentID, "wechat_native", tradeID, 1990, []byte(`{"trade_state":"SUCCESS"}`)); err != nil || !changed {
		t.Fatalf("payment not committed: %v", err)
	}
	if changed, err := store.markPaid(ctx, paymentID, "wechat_native", tradeID, 1990, nil); err != nil || changed {
		t.Fatalf("duplicate payment changed order: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_pay_events WHERE payment_id=?`, paymentID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected exactly one event, got %d: %v", count, err)
	}
	items, err := store.claimDeliveries(ctx, 10)
	if err != nil || len(items) != 1 || items[0].Order.PaymentID != paymentID {
		t.Fatalf("outgoing event not claimed: %+v, %v", items, err)
	}
	if err := store.deliveryResult(ctx, items[0].ID, items[0].Attempts, nil); err != nil {
		t.Fatal(err)
	}
	orders, cursor, more, err := store.list(ctx, clientID, "", 10)
	if err != nil || len(orders) != 1 || orders[0].Status != "paid" || cursor == "" || more {
		t.Fatalf("reconciliation list incorrect: %+v, %s, %v, %v", orders, cursor, more, err)
	}
	orders, _, _, err = store.list(ctx, clientID, cursor, 10)
	if err != nil || len(orders) != 0 {
		t.Fatalf("reconciliation cursor repeated order: %+v, %v", orders, err)
	}
}
