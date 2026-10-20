package externalpay

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	ID              string
	PublicKeyPEM    string
	Enabled         bool
	AllowedChannels string
	MinAmountFen    int64
	MaxAmountFen    int64
	MaxDailyOrders  int
}

type Order struct {
	PaymentID       string     `json:"payment_id"`
	ClientID        string     `json:"client_id"`
	MerchantOrderID string     `json:"merchant_order_id"`
	Channel         string     `json:"channel"`
	AmountFen       int64      `json:"amount_fen"`
	Currency        string     `json:"currency"`
	Subject         string     `json:"subject"`
	CallbackURL     string     `json:"-"`
	Status          string     `json:"status"`
	PayURL          string     `json:"pay_url,omitempty"`
	ProviderTradeID string     `json:"provider_trade_id,omitempty"`
	FailureReason   string     `json:"failure_reason,omitempty"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	id              uint64
}

type Store struct{ db *sql.DB }

func (s *Store) client(ctx context.Context, id string) (Client, error) {
	var c Client
	err := s.db.QueryRowContext(ctx, `SELECT client_id, public_key_pem, enabled, allowed_channels, min_amount_fen, max_amount_fen, max_daily_orders FROM external_pay_clients WHERE client_id=?`, id).Scan(&c.ID, &c.PublicKeyPEM, &c.Enabled, &c.AllowedChannels, &c.MinAmountFen, &c.MaxAmountFen, &c.MaxDailyOrders)
	return c, err
}

func (s *Store) dailyOrderCount(ctx context.Context, clientID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_pay_orders WHERE client_id=? AND created_at>=CURRENT_DATE()`, clientID).Scan(&count)
	return count, err
}

const orderColumns = `id, payment_id, client_id, merchant_order_id, channel, amount_fen, currency, subject, callback_url, status, pay_url, provider_trade_id, failure_reason, paid_at, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scanOrder(row scanner) (Order, error) {
	var o Order
	var payURL, tradeID, failure sql.NullString
	var paidAt sql.NullTime
	err := row.Scan(&o.id, &o.PaymentID, &o.ClientID, &o.MerchantOrderID, &o.Channel, &o.AmountFen, &o.Currency, &o.Subject, &o.CallbackURL, &o.Status, &payURL, &tradeID, &failure, &paidAt, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return o, err
	}
	o.PayURL, o.ProviderTradeID, o.FailureReason = payURL.String, tradeID.String, failure.String
	if paidAt.Valid {
		o.PaidAt = &paidAt.Time
	}
	return o, nil
}

func (s *Store) order(ctx context.Context, paymentID string) (Order, error) {
	return scanOrder(s.db.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM external_pay_orders WHERE payment_id=?`, paymentID))
}

func (s *Store) paymentLinkActive(ctx context.Context, paymentID string) (bool, error) {
	var active bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM external_pay_orders WHERE payment_id=? AND status='pending' AND created_at>CURRENT_TIMESTAMP(6)-INTERVAL 15 MINUTE)`, paymentID).Scan(&active)
	return active, err
}

func (s *Store) merchantOrder(ctx context.Context, clientID, merchantID string) (Order, error) {
	return scanOrder(s.db.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM external_pay_orders WHERE client_id=? AND merchant_order_id=?`, clientID, merchantID))
}

func (s *Store) insertOrder(ctx context.Context, o Order) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO external_pay_orders (payment_id,client_id,merchant_order_id,channel,amount_fen,currency,subject,callback_url,status) VALUES (?,?,?,?,?,?,?,?,?)`, o.PaymentID, o.ClientID, o.MerchantOrderID, o.Channel, o.AmountFen, o.Currency, o.Subject, o.CallbackURL, "creating")
	return err
}

func (s *Store) prepayReady(ctx context.Context, paymentID, payURL string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE external_pay_orders SET pay_url=?,status='pending',failure_reason=NULL WHERE payment_id=? AND status='creating'`, payURL, paymentID)
	return err
}

func (s *Store) prepayFailed(ctx context.Context, paymentID string, failure error) {
	message := failure.Error()
	if len(message) > 255 {
		message = message[:255]
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE external_pay_orders SET status='init_failed',failure_reason=? WHERE payment_id=? AND status='creating'`, message, paymentID)
}

func (s *Store) claimRetry(ctx context.Context, paymentID string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE external_pay_orders SET status='creating',failure_reason=NULL,updated_at=CURRENT_TIMESTAMP(6) WHERE payment_id=? AND (status='init_failed' OR (status='creating' AND updated_at<CURRENT_TIMESTAMP(6)-INTERVAL 1 MINUTE))`, paymentID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// markPaid and its outgoing event commit together. Provider notifications can race or repeat.
func (s *Store) markPaid(ctx context.Context, paymentID, channel, tradeID string, amountFen int64, providerPayload []byte) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var currentChannel, status string
	var expected int64
	err = tx.QueryRowContext(ctx, `SELECT channel,amount_fen,status FROM external_pay_orders WHERE payment_id=? FOR UPDATE`, paymentID).Scan(&currentChannel, &expected, &status)
	if err != nil {
		return false, err
	}
	if currentChannel != channel || expected != amountFen || tradeID == "" {
		return false, errors.New("provider payment does not match order")
	}
	if status == "paid" {
		return false, nil
	}
	if status != "pending" && status != "creating" && status != "init_failed" && status != "closed" {
		return false, fmt.Errorf("order state %s cannot be paid", status)
	}
	_, err = tx.ExecContext(ctx, `UPDATE external_pay_orders SET status='paid',provider_trade_id=?,provider_payload=?,paid_at=CURRENT_TIMESTAMP(6),failure_reason=NULL WHERE payment_id=?`, tradeID, providerPayload, paymentID)
	if err != nil {
		return false, err
	}
	eventID, err := randomID("EV_")
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO external_pay_events (event_id,payment_id,event_type,status) VALUES (?,?,'payment.paid','pending')`, eventID, paymentID)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *Store) markClosed(ctx context.Context, paymentID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE external_pay_orders SET status='closed' WHERE payment_id=? AND status='pending'`, paymentID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (s *Store) updateChecked(ctx context.Context, paymentID string) {
	_, _ = s.db.ExecContext(ctx, `UPDATE external_pay_orders SET checked_at=CURRENT_TIMESTAMP(6) WHERE payment_id=?`, paymentID)
}

func (s *Store) pendingOrders(ctx context.Context, limit int) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+orderColumns+` FROM external_pay_orders WHERE status IN ('pending','creating','init_failed') AND created_at < CURRENT_TIMESTAMP(6)-INTERVAL 30 SECOND AND (checked_at IS NULL OR checked_at < CURRENT_TIMESTAMP(6)-INTERVAL 1 MINUTE) ORDER BY checked_at,created_at LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []Order
	for rows.Next() {
		o, e := scanOrder(rows)
		if e != nil {
			return nil, e
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (s *Store) list(ctx context.Context, clientID, cursor string, limit int) ([]Order, string, bool, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if cursor == "" {
		rows, err = s.db.QueryContext(ctx, `SELECT `+orderColumns+` FROM external_pay_orders WHERE client_id=? ORDER BY updated_at,id LIMIT ?`, clientID, limit+1)
	} else {
		decoded, e := base64.RawURLEncoding.DecodeString(cursor)
		if e != nil {
			return nil, "", false, errors.New("invalid cursor")
		}
		parts := strings.Split(string(decoded), "|")
		if len(parts) != 2 {
			return nil, "", false, errors.New("invalid cursor")
		}
		_, e = time.Parse("2006-01-02 15:04:05.999999", parts[0])
		if e != nil {
			return nil, "", false, errors.New("invalid cursor")
		}
		id, e := strconv.ParseUint(parts[1], 10, 64)
		if e != nil {
			return nil, "", false, errors.New("invalid cursor")
		}
		rows, err = s.db.QueryContext(ctx, `SELECT `+orderColumns+` FROM external_pay_orders WHERE client_id=? AND (updated_at>? OR (updated_at=? AND id>?)) ORDER BY updated_at,id LIMIT ?`, clientID, parts[0], parts[0], id, limit+1)
	}
	if err != nil {
		return nil, "", false, err
	}
	defer rows.Close()
	var orders []Order
	for rows.Next() {
		o, e := scanOrder(rows)
		if e != nil {
			return nil, "", false, e
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, "", false, err
	}
	hasMore := len(orders) > limit
	if hasMore {
		orders = orders[:limit]
	}
	if len(orders) == 0 {
		return orders, cursor, false, nil
	}
	last := orders[len(orders)-1]
	next := base64.RawURLEncoding.EncodeToString([]byte(last.UpdatedAt.Format("2006-01-02 15:04:05.999999") + "|" + strconv.FormatUint(last.id, 10)))
	return orders, next, hasMore, nil
}

type delivery struct {
	ID       uint64
	EventID  string
	Order    Order
	Attempts int
}

func (s *Store) claimDeliveries(ctx context.Context, limit int) ([]delivery, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT e.id,e.event_id,e.attempts,o.payment_id FROM external_pay_events e JOIN external_pay_orders o ON o.payment_id=e.payment_id WHERE (e.status='pending' AND e.next_attempt_at<=CURRENT_TIMESTAMP(6)) OR (e.status='sending' AND e.locked_until<CURRENT_TIMESTAMP(6)) ORDER BY e.id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []delivery
	for rows.Next() {
		var d delivery
		var paymentID string
		if err := rows.Scan(&d.ID, &d.EventID, &d.Attempts, &paymentID); err != nil {
			return nil, err
		}
		d.Order.PaymentID = paymentID
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var claimed []delivery
	for _, item := range items {
		res, e := s.db.ExecContext(ctx, `UPDATE external_pay_events SET status='sending',locked_until=CURRENT_TIMESTAMP(6)+INTERVAL 30 SECOND,attempts=attempts+1 WHERE id=? AND ((status='pending' AND next_attempt_at<=CURRENT_TIMESTAMP(6)) OR (status='sending' AND locked_until<CURRENT_TIMESTAMP(6)))`, item.ID)
		if e != nil {
			return claimed, e
		}
		n, _ := res.RowsAffected()
		if n == 1 {
			item.Order, e = s.order(ctx, item.Order.PaymentID)
			if e != nil {
				return claimed, e
			}
			item.Attempts++
			claimed = append(claimed, item)
		}
	}
	return claimed, nil
}

func (s *Store) deliveryResult(ctx context.Context, id uint64, attempts int, sendErr error) error {
	if sendErr == nil {
		_, err := s.db.ExecContext(ctx, `UPDATE external_pay_events SET status='delivered',delivered_at=CURRENT_TIMESTAMP(6),locked_until=NULL,last_error=NULL WHERE id=?`, id)
		return err
	}
	message := sendErr.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	if attempts >= 20 {
		_, err := s.db.ExecContext(ctx, `UPDATE external_pay_events SET status='dead',locked_until=NULL,last_error=? WHERE id=?`, message, id)
		return err
	}
	delay := 1 << min(attempts, 10)
	_, err := s.db.ExecContext(ctx, `UPDATE external_pay_events SET status='pending',locked_until=NULL,last_error=?,next_attempt_at=CURRENT_TIMESTAMP(6)+INTERVAL ? SECOND WHERE id=?`, message, delay, id)
	return err
}
