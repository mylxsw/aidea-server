package externalpay

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/payment/wechatpay"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
)

var clientIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var merchantIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,100}$`)

type Service struct {
	store      *Store
	conf       *config.Config
	gateway    paymentGateway
	signer     ed25519.PrivateKey
	httpClient *http.Client
}

// paymentGateway keeps the HTTP/order flow testable without calling live payment providers.
type paymentGateway interface {
	prepay(context.Context, Order) (string, error)
	alipayNotification(*http.Request) (verifiedPayment, error)
	wechatNotification(context.Context, *http.Request) (verifiedPayment, error)
	query(context.Context, Order) (verifiedPayment, error)
	close(context.Context, Order) error
}

type Provider struct{}

func (Provider) Register(binder infra.Binder) { binder.MustSingleton(NewService) }

func NewService(db *sql.DB, conf *config.Config, wx wechatpay.WeChatPay) (*Service, error) {
	s := &Service{store: &Store{db: db}, conf: conf, httpClient: webhookClient()}
	if !conf.ExternalPayEnabled {
		return s, nil
	}
	base, err := url.Parse(conf.ExternalPayBaseURL)
	if err != nil || base.Path != "" || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("external payment base URL must be an HTTPS origin without path or query")
	}
	if _, err := validateCallbackURL(strings.TrimRight(conf.ExternalPayBaseURL, "/") + "/partner/v1/payments/callback/wechat"); err != nil {
		return nil, fmt.Errorf("invalid external payment base URL: %w", err)
	}
	if conf.ExternalPayPrivateKeyPath == "" {
		return nil, errors.New("external payment signing key not configured")
	}
	key, err := loadPrivateKey(conf.ExternalPayPrivateKeyPath)
	if err != nil {
		return nil, err
	}
	s.signer = key
	gateway, err := newGateway(conf, wx)
	if err != nil {
		return nil, err
	}
	s.gateway = gateway
	return s, nil
}

type createRequest struct {
	MerchantOrderID string `json:"merchant_order_id"`
	Channel         string `json:"channel"`
	AmountFen       int64  `json:"amount_fen"`
	Currency        string `json:"currency"`
	Subject         string `json:"subject"`
	CallbackURL     string `json:"callback_url"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func (s *Service) Handler() http.Handler { return http.HandlerFunc(s.serveHTTP) }

func (s *Service) serveHTTP(w http.ResponseWriter, r *http.Request) {
	const root = "/partner/v1/payments"
	path := strings.TrimPrefix(r.URL.Path, root)
	switch path {
	case "/docs":
		if r.Method != http.MethodGet {
			writeError(w, 405, "method_not_allowed", "GET required")
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = io.WriteString(w, apiDocumentation)
		return
	}
	if !s.conf.ExternalPayEnabled {
		http.NotFound(w, r)
		return
	}
	switch path {
	case "/public-key":
		if r.Method != http.MethodGet {
			writeError(w, 405, "method_not_allowed", "GET required")
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = io.WriteString(w, publicKeyPEM(s.signer))
		return
	case "/return":
		if r.Method != http.MethodGet {
			writeError(w, 405, "method_not_allowed", "GET required")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<html><body>请返回应用查看支付结果。</body></html>")
		return
	case "/callback/alipay":
		s.handleAlipay(w, r)
		return
	case "/callback/wechat":
		s.handleWechat(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasSuffix(path, "/pay") && strings.Count(strings.Trim(path, "/"), "/") == 1 {
		s.openPay(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/pay"))
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", "unsupported method")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 413, "request_too_large", "request body too large")
		return
	}
	client, err := s.authenticate(r, body)
	if err != nil {
		writeError(w, 401, "unauthorized", err.Error())
		return
	}
	if path == "" || path == "/" {
		if r.Method == http.MethodPost {
			s.create(w, r, client, body)
		} else {
			s.list(w, r, client)
		}
		return
	}
	if r.Method == http.MethodGet && strings.Count(strings.Trim(path, "/"), "/") == 0 {
		s.get(w, r, client, strings.Trim(path, "/"))
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(path, "/close") && strings.Count(strings.Trim(path, "/"), "/") == 1 {
		s.close(w, r, client, strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/close"))
		return
	}
	writeError(w, 404, "not_found", "unknown endpoint")
}

func (s *Service) authenticate(r *http.Request, body []byte) (Client, error) {
	var zero Client
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return zero, errors.New("missing bearer token")
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return zero, errors.New("invalid JWT")
	}
	decoded, err := jwtEncoding.DecodeString(parts[1])
	if err != nil {
		return zero, errors.New("invalid JWT")
	}
	var unsafe struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(decoded, &unsafe); err != nil || !clientIDPattern.MatchString(unsafe.Issuer) {
		return zero, errors.New("invalid client ID")
	}
	client, err := s.store.client(r.Context(), unsafe.Issuer)
	if err != nil || !client.Enabled {
		return zero, errors.New("unknown or disabled client")
	}
	public, err := parsePublicKey(client.PublicKeyPEM)
	if err != nil {
		return zero, errors.New("invalid client public key")
	}
	claims, err := verifyJWT(token, public, "aidea-external-pay", time.Now())
	if err != nil {
		return zero, err
	}
	if claims.Issuer != client.ID || claims.Method != r.Method || claims.Path != r.URL.RequestURI() || claims.BodySHA256 != bodyDigest(body) {
		return zero, errors.New("JWT request binding mismatch")
	}
	_, err = s.store.db.ExecContext(r.Context(), `INSERT INTO external_pay_nonces (client_id,nonce,expires_at) VALUES (?,?,FROM_UNIXTIME(?))`, client.ID, claims.ID, claims.ExpiresAt)
	if err != nil {
		return zero, errors.New("replayed JWT or nonce store unavailable")
	}
	return client, nil
}

func (s *Service) create(w http.ResponseWriter, r *http.Request, client Client, body []byte) {
	var input createRequest
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "invalid JSON")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, 400, "invalid_request", "unexpected JSON content")
		return
	}
	if !merchantIDPattern.MatchString(input.MerchantOrderID) || utf8.RuneCountInString(input.Subject) < 1 || utf8.RuneCountInString(input.Subject) > 127 || strings.ContainsAny(input.Subject, "\r\n\x00") {
		writeError(w, 400, "invalid_request", "invalid order ID or subject")
		return
	}
	if input.Currency == "" {
		input.Currency = "CNY"
	}
	if input.Currency != "CNY" || input.AmountFen < client.MinAmountFen || input.AmountFen > client.MaxAmountFen || input.AmountFen <= 0 {
		writeError(w, 400, "invalid_amount", "unsupported amount or currency")
		return
	}
	if input.Channel != "alipay_page" && input.Channel != "wechat_native" {
		writeError(w, 400, "invalid_channel", "unsupported payment channel")
		return
	}
	allowed := false
	for _, channel := range strings.Split(client.AllowedChannels, ",") {
		if strings.TrimSpace(channel) == input.Channel {
			allowed = true
			break
		}
	}
	if !allowed {
		writeError(w, 403, "channel_disabled", "channel is not enabled for this client")
		return
	}
	if _, err := validateCallbackURL(input.CallbackURL); err != nil {
		writeError(w, 400, "invalid_callback", err.Error())
		return
	}
	if old, err := s.store.merchantOrder(r.Context(), client.ID, input.MerchantOrderID); err == nil {
		if !sameOrder(old, input) {
			writeError(w, 409, "order_conflict", "merchant order ID already has different parameters")
			return
		}
		if old.Status == "init_failed" || old.Status == "creating" {
			claimed, err := s.store.claimRetry(r.Context(), old.PaymentID)
			if err != nil {
				writeError(w, 500, "storage_error", "could not retry order")
				return
			}
			if claimed {
				s.finishPrepay(w, r, old, 200)
				return
			}
		}
		writeJSON(w, 200, s.publicOrder(old))
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, 500, "storage_error", "could not read order")
		return
	}
	count, err := s.store.dailyOrderCount(r.Context(), client.ID)
	if err != nil {
		writeError(w, 500, "storage_error", "could not check order limit")
		return
	}
	if count >= client.MaxDailyOrders {
		writeError(w, 429, "daily_limit", "daily order limit reached")
		return
	}
	id, err := randomPaymentID()
	if err != nil {
		writeError(w, 500, "internal_error", "could not create payment ID")
		return
	}
	o := Order{PaymentID: id, ClientID: client.ID, MerchantOrderID: input.MerchantOrderID, Channel: input.Channel, AmountFen: input.AmountFen, Currency: input.Currency, Subject: input.Subject, CallbackURL: input.CallbackURL, Status: "creating"}
	if err := s.store.insertOrder(r.Context(), o); err != nil {
		if old, e := s.store.merchantOrder(r.Context(), client.ID, input.MerchantOrderID); e == nil && sameOrder(old, input) {
			writeJSON(w, 200, s.publicOrder(old))
			return
		}
		writeError(w, 500, "storage_error", "could not create order")
		return
	}
	created, err := s.store.order(r.Context(), o.PaymentID)
	if err != nil {
		writeError(w, 500, "storage_error", "could not read new order")
		return
	}
	s.finishPrepay(w, r, created, 201)
}

func (s *Service) finishPrepay(w http.ResponseWriter, r *http.Request, o Order, successCode int) {
	payURL, err := s.gateway.prepay(r.Context(), o)
	storeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err != nil {
		s.store.prepayFailed(storeCtx, o.PaymentID, err)
		log.Errorf("external payment prepay %s failed: %v", o.PaymentID, err)
		writeError(w, 502, "provider_error", "payment provider unavailable; retry with the same merchant order ID")
		return
	}
	if err := s.store.prepayReady(storeCtx, o.PaymentID, payURL); err != nil {
		writeError(w, 500, "storage_error", "could not save payment link")
		return
	}
	created, err := s.store.order(r.Context(), o.PaymentID)
	if err != nil {
		writeError(w, 500, "storage_error", "could not read new order")
		return
	}
	writeJSON(w, successCode, s.publicOrder(created))
}

func (s *Service) publicOrder(o Order) Order {
	if o.Channel == "alipay_page" && o.Status == "pending" && o.PayURL != "" {
		o.PayURL = strings.TrimRight(s.conf.ExternalPayBaseURL, "/") + "/partner/v1/payments/" + url.PathEscape(o.PaymentID) + "/pay"
	}
	return o
}

func (s *Service) openPay(w http.ResponseWriter, r *http.Request, id string) {
	if !clientIDPattern.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	o, err := s.store.order(r.Context(), id)
	if err != nil || o.Channel != "alipay_page" || o.PayURL == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	active, err := s.store.paymentLinkActive(r.Context(), id)
	if err != nil {
		writeError(w, 500, "storage_error", "could not check payment link")
		return
	}
	if !active {
		writeError(w, 410, "payment_closed", "payment link is no longer available")
		return
	}
	link, err := url.Parse(o.PayURL)
	if err != nil || link.Scheme != "https" || (link.Hostname() != "openapi.alipay.com" && link.Hostname() != "openapi.alipaydev.com") {
		writeError(w, 502, "provider_error", "payment link is unavailable")
		return
	}
	http.Redirect(w, r, link.String(), http.StatusFound)
}

func sameOrder(o Order, input createRequest) bool {
	return o.MerchantOrderID == input.MerchantOrderID && o.Channel == input.Channel && o.AmountFen == input.AmountFen && o.Currency == input.Currency && o.Subject == input.Subject && o.CallbackURL == input.CallbackURL
}

func (s *Service) get(w http.ResponseWriter, r *http.Request, c Client, id string) {
	o, err := s.store.order(r.Context(), id)
	if err != nil || o.ClientID != c.ID {
		writeError(w, 404, "not_found", "order not found")
		return
	}
	writeJSON(w, 200, s.publicOrder(o))
}

func (s *Service) close(w http.ResponseWriter, r *http.Request, c Client, id string) {
	o, err := s.store.order(r.Context(), id)
	if err != nil || o.ClientID != c.ID {
		writeError(w, 404, "not_found", "order not found")
		return
	}
	if o.Status == "closed" {
		writeJSON(w, 200, s.publicOrder(o))
		return
	}
	if o.Status != "pending" {
		writeError(w, 409, "invalid_state", "only pending orders can be closed")
		return
	}
	p, err := s.gateway.query(r.Context(), o)
	if err == nil && (p.Status == "SUCCESS" || p.Status == "TRADE_SUCCESS" || p.Status == "TRADE_FINISHED") {
		if _, err := s.store.markPaid(r.Context(), p.PaymentID, p.Channel, p.TradeID, p.AmountFen, p.Payload); err != nil {
			writeError(w, 502, "provider_mismatch", "payment confirmation does not match order")
			return
		}
		writeError(w, 409, "already_paid", "payment already completed")
		return
	}
	if err != nil {
		writeError(w, 502, "provider_error", "could not verify payment state")
		return
	}
	if p.Status != "TRADE_CLOSED" && p.Status != "CLOSED" && p.Status != "NOT_EXIST" {
		if err := s.gateway.close(r.Context(), o); err != nil {
			writeError(w, 502, "provider_error", "could not close provider order")
			return
		}
	}
	changed, err := s.store.markClosed(r.Context(), id)
	if err != nil {
		writeError(w, 500, "storage_error", "could not close local order")
		return
	}
	closed, err := s.store.order(r.Context(), id)
	if err != nil {
		writeError(w, 500, "storage_error", "could not read order")
		return
	}
	if !changed && closed.Status == "paid" {
		writeError(w, 409, "already_paid", "payment already completed")
		return
	}
	if closed.Status != "closed" {
		writeError(w, 409, "invalid_state", "payment state changed during closure")
		return
	}
	writeJSON(w, 200, s.publicOrder(closed))
}

func (s *Service) list(w http.ResponseWriter, r *http.Request, c Client) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	orders, next, hasMore, err := s.store.list(r.Context(), c.ID, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeError(w, 400, "invalid_cursor", err.Error())
		return
	}
	if orders == nil {
		orders = []Order{}
	}
	for i := range orders {
		orders[i] = s.publicOrder(orders[i])
	}
	writeJSON(w, 200, map[string]any{"orders": orders, "next_cursor": next, "has_more": hasMore})
}

func (s *Service) handleAlipay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", "POST required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	p, err := s.gateway.alipayNotification(r)
	if err != nil {
		writeError(w, 400, "invalid_notification", err.Error())
		return
	}
	if p.Status != "TRADE_SUCCESS" && p.Status != "TRADE_FINISHED" {
		_, _ = io.WriteString(w, "success")
		return
	}
	if !strings.HasPrefix(p.PaymentID, "EX_") {
		writeError(w, 400, "wrong_order_type", "not an external payment")
		return
	}
	if _, err := s.store.markPaid(r.Context(), p.PaymentID, p.Channel, p.TradeID, p.AmountFen, p.Payload); err != nil {
		log.Errorf("external alipay notification rejected: %v", err)
		writeError(w, 400, "order_mismatch", "payment does not match order")
		return
	}
	_, _ = io.WriteString(w, "success")
}

func (s *Service) handleWechat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", "POST required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	p, err := s.gateway.wechatNotification(r.Context(), r)
	if err != nil {
		writeError(w, 400, "invalid_notification", err.Error())
		return
	}
	if !strings.HasPrefix(p.PaymentID, "EX_") {
		writeError(w, 400, "wrong_order_type", "not an external payment")
		return
	}
	if _, err := s.store.markPaid(r.Context(), p.PaymentID, p.Channel, p.TradeID, p.AmountFen, p.Payload); err != nil {
		log.Errorf("external wechat notification rejected: %v", err)
		writeError(w, 400, "order_mismatch", "payment does not match order")
		return
	}
	writeJSON(w, 200, map[string]string{"code": "SUCCESS", "message": "成功"})
}

func DispatchJob(ctx context.Context, s *Service) error {
	if !s.conf.ExternalPayEnabled {
		return nil
	}
	items, err := s.store.claimDeliveries(ctx, 20)
	if err != nil {
		return err
	}
	for _, item := range items {
		id, err := randomID("WH_")
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		token, err := signJWT(s.signer, claims{Issuer: "aidea-external-pay", Audience: item.Order.ClientID, IssuedAt: now, NotBefore: now, ExpiresAt: now + 120, ID: id, EventID: item.EventID, PaymentID: item.Order.PaymentID, MerchantOrderID: item.Order.MerchantOrderID, Channel: item.Order.Channel, AmountFen: item.Order.AmountFen, Currency: item.Order.Currency, Status: item.Order.Status})
		if err != nil {
			return err
		}
		err = postWebhook(ctx, s.httpClient, item.Order.CallbackURL, token)
		if e := s.store.deliveryResult(ctx, item.ID, item.Attempts, err); e != nil {
			return e
		}
	}
	_, _ = s.store.db.ExecContext(ctx, `DELETE FROM external_pay_nonces WHERE expires_at<CURRENT_TIMESTAMP(6)-INTERVAL 5 MINUTE LIMIT 1000`)
	return nil
}

func ReconcileJob(ctx context.Context, s *Service) error {
	if !s.conf.ExternalPayEnabled {
		return nil
	}
	orders, err := s.store.pendingOrders(ctx, 20)
	if err != nil {
		return err
	}
	for _, o := range orders {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		p, queryErr := s.gateway.query(checkCtx, o)
		cancel()
		s.store.updateChecked(ctx, o.PaymentID)
		if queryErr != nil {
			log.Errorf("external payment query %s failed: %v", o.PaymentID, queryErr)
			continue
		}
		if p.Status == "SUCCESS" || p.Status == "TRADE_SUCCESS" || p.Status == "TRADE_FINISHED" {
			if _, err := s.store.markPaid(ctx, p.PaymentID, p.Channel, p.TradeID, p.AmountFen, p.Payload); err != nil {
				log.Errorf("external payment reconcile %s failed: %v", o.PaymentID, err)
			}
		}
	}
	return nil
}
