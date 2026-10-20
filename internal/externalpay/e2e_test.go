package externalpay

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mylxsw/aidea-server/config"
)

type e2eGateway struct {
	mu         sync.Mutex
	prepayIDs  []string
	closeIDs   []string
	queryState map[string]verifiedPayment
}

func (g *e2eGateway) prepay(_ context.Context, o Order) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.prepayIDs = append(g.prepayIDs, o.PaymentID)
	if o.Channel == "alipay_page" {
		return "https://openapi.alipay.com/gateway.do?order=test", nil
	}
	return "weixin://wxpay/" + o.PaymentID, nil
}

func (g *e2eGateway) alipayNotification(*http.Request) (verifiedPayment, error) {
	return verifiedPayment{}, errors.New("alipay provider not configured in this test")
}

func (g *e2eGateway) wechatNotification(_ context.Context, r *http.Request) (verifiedPayment, error) {
	var notice struct {
		PaymentID string `json:"payment_id"`
		TradeID   string `json:"trade_id"`
		AmountFen int64  `json:"amount_fen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&notice); err != nil {
		return verifiedPayment{}, err
	}
	return verifiedPayment{PaymentID: notice.PaymentID, TradeID: notice.TradeID, AmountFen: notice.AmountFen, Channel: "wechat_native", Status: "SUCCESS", Payload: []byte(`{"provider":"test"}`)}, nil
}

func (g *e2eGateway) query(_ context.Context, o Order) (verifiedPayment, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if p, ok := g.queryState[o.PaymentID]; ok {
		return p, nil
	}
	return verifiedPayment{PaymentID: o.PaymentID, Channel: o.Channel, Status: "NOTPAY"}, nil
}

func (g *e2eGateway) close(_ context.Context, o Order) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closeIDs = append(g.closeIDs, o.PaymentID)
	return nil
}

// rewriteCallbackTransport keeps the application-facing callback HTTPS URL intact while
// routing the test HTTP client to a local TLS receiver. Production uses webhookClient.
type rewriteCallbackTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t rewriteCallbackTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return t.base.RoundTrip(clone)
}

func e2ePublicKeyPEM(t *testing.T, key ed25519.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func TestExternalPaymentEndToEnd(t *testing.T) {
	db := externalPayTestDB(t)
	ctx := context.Background()
	var err error
	clientID, err := randomID("app_")
	if err != nil {
		t.Fatal(err)
	}
	otherID, err := randomID("app_")
	if err != nil {
		t.Fatal(err)
	}
	clientPublic, clientPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPublic, otherPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		id  string
		key ed25519.PublicKey
	}{{clientID, clientPublic}, {otherID, otherPublic}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO external_pay_clients (client_id,public_key_pem) VALUES (?,?)`, entry.id, e2ePublicKeyPEM(t, entry.key)); err != nil {
			t.Fatal(err)
		}
	}
	serverPublic, serverPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gateway := &e2eGateway{queryState: map[string]verifiedPayment{}}
	var notificationMu sync.Mutex
	var notifications []claims
	var callbackPaths []string
	firstFailure := true
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		token, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		got, err := verifyJWT(string(token), serverPublic, clientID, time.Now())
		if err != nil || got.Issuer != "aidea-external-pay" || got.Status != "paid" || got.AmountFen != 1990 {
			t.Errorf("invalid outbound payment notification: %+v, %v", got, err)
			w.WriteHeader(400)
			return
		}
		notificationMu.Lock()
		notifications = append(notifications, got)
		callbackPaths = append(callbackPaths, r.URL.Path)
		fail := firstFailure
		firstFailure = false
		notificationMu.Unlock()
		if fail {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(204)
	}))
	defer receiver.Close()
	receiverURL, err := url.Parse(receiver.URL)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{
		store:      &Store{db: db},
		conf:       &config.Config{ExternalPayEnabled: true, ExternalPayBaseURL: "https://pay.example.com"},
		gateway:    gateway,
		signer:     serverPrivate,
		httpClient: &http.Client{Transport: rewriteCallbackTransport{target: receiverURL, base: receiver.Client().Transport}, Timeout: 5 * time.Second},
	}
	api := httptest.NewServer(service.Handler())
	defer api.Close()
	call := func(method, path string, body []byte, id string, key ed25519.PrivateKey) (*http.Response, []byte) {
		t.Helper()
		now := time.Now().Unix()
		jti, err := randomID("request_")
		if err != nil {
			t.Fatal(err)
		}
		token, err := signJWT(key, claims{Issuer: id, Audience: "aidea-external-pay", IssuedAt: now, NotBefore: now, ExpiresAt: now + 60, ID: jti, Method: method, Path: path, BodySHA256: bodyDigest(body)})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(method, api.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := api.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return res, data
	}
	create := func(merchant, callback string) Order {
		t.Helper()
		body, err := json.Marshal(createRequest{MerchantOrderID: merchant, Channel: "wechat_native", AmountFen: 1990, Currency: "CNY", Subject: "test subscription", CallbackURL: callback})
		if err != nil {
			t.Fatal(err)
		}
		res, data := call("POST", "/partner/v1/payments", body, clientID, clientPrivate)
		if res.StatusCode != 201 {
			t.Fatalf("create HTTP %d: %s", res.StatusCode, data)
		}
		var order Order
		if err := json.Unmarshal(data, &order); err != nil {
			t.Fatal(err)
		}
		if order.PaymentID == "" || order.Status != "pending" || !strings.HasPrefix(order.PayURL, "weixin://") {
			t.Fatalf("invalid created order: %+v", order)
		}
		return order
	}
	first := create("business-1", "https://callback.example.com/first")
	firstTradeID, err := randomID("wx_")
	if err != nil {
		t.Fatal(err)
	}
	if persisted, err := service.store.order(ctx, first.PaymentID); err != nil || persisted.CallbackURL != "https://callback.example.com/first" {
		t.Fatalf("dynamic callback address was not saved: %+v, %v", persisted, err)
	}
	identical, _ := json.Marshal(createRequest{MerchantOrderID: "business-1", Channel: "wechat_native", AmountFen: 1990, Currency: "CNY", Subject: "test subscription", CallbackURL: "https://callback.example.com/first"})
	res, data := call("POST", "/partner/v1/payments", identical, clientID, clientPrivate)
	if res.StatusCode != 200 || !bytes.Contains(data, []byte(first.PaymentID)) || len(gateway.prepayIDs) != 1 {
		t.Fatalf("idempotent create failed: HTTP %d, %s", res.StatusCode, data)
	}
	conflict := bytes.Replace(identical, []byte(`"amount_fen":1990`), []byte(`"amount_fen":2000`), 1)
	res, _ = call("POST", "/partner/v1/payments", conflict, clientID, clientPrivate)
	if res.StatusCode != 409 {
		t.Fatalf("different amount was accepted for same merchant order: HTTP %d", res.StatusCode)
	}
	res, _ = call("GET", "/partner/v1/payments/"+first.PaymentID, nil, otherID, otherPrivate)
	if res.StatusCode != 404 {
		t.Fatalf("cross-client order access succeeded: HTTP %d", res.StatusCode)
	}
	// A signed request can be used once only, even if every request byte is identical.
	now := time.Now().Unix()
	replayToken, err := signJWT(clientPrivate, claims{Issuer: clientID, Audience: "aidea-external-pay", IssuedAt: now, NotBefore: now, ExpiresAt: now + 60, ID: "same-nonce", Method: "GET", Path: "/partner/v1/payments/" + first.PaymentID, BodySHA256: bodyDigest(nil)})
	if err != nil {
		t.Fatal(err)
	}
	for attempt, expected := range []int{200, 401} {
		req, err := http.NewRequest("GET", api.URL+"/partner/v1/payments/"+first.PaymentID, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+replayToken)
		response, err := api.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != expected {
			t.Fatalf("replay attempt %d: HTTP %d, expected %d", attempt+1, response.StatusCode, expected)
		}
	}
	res, _ = call("POST", "/partner/v1/payments", []byte(`{"merchant_order_id":"bad","channel":"wechat_native","amount_fen":1990,"subject":"test","callback_url":"http://localhost/notify"}`), clientID, clientPrivate)
	if res.StatusCode != 400 {
		t.Fatalf("unsafe callback accepted: HTTP %d", res.StatusCode)
	}
	providerCallback := func(paymentID, tradeID string, amount int64) int {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"payment_id": paymentID, "trade_id": tradeID, "amount_fen": amount})
		res, err := api.Client().Post(api.URL+"/partner/v1/payments/callback/wechat", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if code := providerCallback(first.PaymentID, firstTradeID, 1991); code != 400 {
		t.Fatalf("wrong provider amount accepted: HTTP %d", code)
	}
	if code := providerCallback("legacy-internal-order", firstTradeID, 1990); code != 400 {
		t.Fatalf("legacy order entered external callback: HTTP %d", code)
	}
	if code := providerCallback(first.PaymentID, firstTradeID, 1990); code != 200 {
		t.Fatalf("valid provider callback rejected: HTTP %d", code)
	}
	if code := providerCallback(first.PaymentID, firstTradeID, 1990); code != 200 {
		t.Fatalf("duplicate provider callback rejected: HTTP %d", code)
	}
	var eventCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_pay_events WHERE payment_id=?`, first.PaymentID).Scan(&eventCount); err != nil || eventCount != 1 {
		t.Fatalf("duplicate outgoing events: %d, %v", eventCount, err)
	}
	res, data = call("GET", "/partner/v1/payments/"+first.PaymentID, nil, clientID, clientPrivate)
	if res.StatusCode != 200 || !bytes.Contains(data, []byte(`"status":"paid"`)) {
		t.Fatalf("paid order not queryable: HTTP %d, %s", res.StatusCode, data)
	}
	if err := DispatchJob(ctx, service); err != nil {
		t.Fatal(err)
	}
	var eventStatus string
	if err := db.QueryRowContext(ctx, `SELECT status FROM external_pay_events WHERE payment_id=?`, first.PaymentID).Scan(&eventStatus); err != nil || eventStatus != "pending" {
		t.Fatalf("failed webhook did not remain pending: %s, %v", eventStatus, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE external_pay_events SET next_attempt_at=CURRENT_TIMESTAMP(6)-INTERVAL 1 SECOND WHERE payment_id=?`, first.PaymentID); err != nil {
		t.Fatal(err)
	}
	if err := DispatchJob(ctx, service); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT status FROM external_pay_events WHERE payment_id=?`, first.PaymentID).Scan(&eventStatus); err != nil || eventStatus != "delivered" {
		t.Fatalf("retried webhook not delivered: %s, %v", eventStatus, err)
	}
	notificationMu.Lock()
	if len(notifications) != 2 || notifications[0].EventID != notifications[1].EventID || callbackPaths[0] != "/first" || callbackPaths[1] != "/first" {
		t.Errorf("notification retry did not preserve event ID and destination: %+v, %+v", notifications, callbackPaths)
	}
	notificationMu.Unlock()
	res, data = call("GET", "/partner/v1/payments?limit=1", nil, clientID, clientPrivate)
	if res.StatusCode != 200 {
		t.Fatalf("list HTTP %d: %s", res.StatusCode, data)
	}
	var listResult struct {
		Orders     []Order `json:"orders"`
		NextCursor string  `json:"next_cursor"`
	}
	if err := json.Unmarshal(data, &listResult); err != nil || len(listResult.Orders) != 1 || listResult.NextCursor == "" {
		t.Fatalf("reconciliation list invalid: %s, %v", data, err)
	}
	missed := create("business-2", "https://callback.example.com/second")
	secondTradeID, err := randomID("wx_")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE external_pay_orders SET created_at=CURRENT_TIMESTAMP(6)-INTERVAL 1 MINUTE WHERE payment_id=?`, missed.PaymentID); err != nil {
		t.Fatal(err)
	}
	gateway.mu.Lock()
	gateway.queryState[missed.PaymentID] = verifiedPayment{PaymentID: missed.PaymentID, TradeID: secondTradeID, Channel: "wechat_native", AmountFen: 1990, Status: "SUCCESS"}
	gateway.mu.Unlock()
	if err := ReconcileJob(ctx, service); err != nil {
		t.Fatal(err)
	}
	if reconciled, err := service.store.order(ctx, missed.PaymentID); err != nil || reconciled.Status != "paid" {
		t.Fatalf("missed provider callback was not recovered: %+v, %v", reconciled, err)
	}
	if err := DispatchJob(ctx, service); err != nil {
		t.Fatal(err)
	}
	notificationMu.Lock()
	if len(callbackPaths) != 3 || callbackPaths[2] != "/second" {
		t.Errorf("second order did not use its own callback URL: %+v", callbackPaths)
	}
	notificationMu.Unlock()
	open := create("business-3", "https://callback.example.com/third")
	res, data = call("POST", fmt.Sprintf("/partner/v1/payments/%s/close", open.PaymentID), nil, clientID, clientPrivate)
	if res.StatusCode != 200 || !bytes.Contains(data, []byte(`"status":"closed"`)) {
		t.Fatalf("unpaid order was not closed: HTTP %d, %s", res.StatusCode, data)
	}
	if len(gateway.closeIDs) != 1 || gateway.closeIDs[0] != open.PaymentID {
		t.Fatalf("provider close not called: %+v", gateway.closeIDs)
	}
	alipayBody, _ := json.Marshal(createRequest{MerchantOrderID: "business-alipay", Channel: "alipay_page", AmountFen: 1990, Currency: "CNY", Subject: "test subscription", CallbackURL: "https://callback.example.com/alipay"})
	res, data = call("POST", "/partner/v1/payments", alipayBody, clientID, clientPrivate)
	if res.StatusCode != 201 {
		t.Fatalf("alipay create HTTP %d: %s", res.StatusCode, data)
	}
	var alipayOrder Order
	if err := json.Unmarshal(data, &alipayOrder); err != nil {
		t.Fatal(err)
	}
	path := "/partner/v1/payments/" + alipayOrder.PaymentID + "/pay"
	if alipayOrder.PayURL != "https://pay.example.com"+path {
		t.Fatalf("Alipay signed URL was exposed: %s", alipayOrder.PayURL)
	}
	redirectClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	redirect, err := redirectClient.Get(api.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	redirect.Body.Close()
	if redirect.StatusCode != 302 || redirect.Header.Get("Location") != "https://openapi.alipay.com/gateway.do?order=test" {
		t.Fatalf("active Alipay link not redirected: HTTP %d", redirect.StatusCode)
	}
	gateway.mu.Lock()
	gateway.queryState[alipayOrder.PaymentID] = verifiedPayment{PaymentID: alipayOrder.PaymentID, Channel: "alipay_page", Status: "NOT_EXIST"}
	gateway.mu.Unlock()
	res, data = call("POST", strings.TrimSuffix(path, "/pay")+"/close", nil, clientID, clientPrivate)
	if res.StatusCode != 200 || !bytes.Contains(data, []byte(`"status":"closed"`)) || len(gateway.closeIDs) != 1 {
		t.Fatalf("Alipay trade-not-exist closure failed: HTTP %d, %s", res.StatusCode, data)
	}
	redirect, err = redirectClient.Get(api.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	redirect.Body.Close()
	if redirect.StatusCode != 410 {
		t.Fatalf("closed Alipay link remained usable: HTTP %d", redirect.StatusCode)
	}
	expiredBody, _ := json.Marshal(createRequest{MerchantOrderID: "business-alipay-expired", Channel: "alipay_page", AmountFen: 1990, Currency: "CNY", Subject: "expired subscription", CallbackURL: "https://callback.example.com/alipay"})
	res, data = call("POST", "/partner/v1/payments", expiredBody, clientID, clientPrivate)
	if res.StatusCode != 201 {
		t.Fatalf("expired Alipay test order creation HTTP %d: %s", res.StatusCode, data)
	}
	var expiredOrder Order
	if err := json.Unmarshal(data, &expiredOrder); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE external_pay_orders SET created_at=CURRENT_TIMESTAMP(6)-INTERVAL 16 MINUTE WHERE payment_id=?`, expiredOrder.PaymentID); err != nil {
		t.Fatal(err)
	}
	redirect, err = redirectClient.Get(api.URL + "/partner/v1/payments/" + expiredOrder.PaymentID + "/pay")
	if err != nil {
		t.Fatal(err)
	}
	redirect.Body.Close()
	if redirect.StatusCode != 410 {
		t.Fatalf("expired Alipay link remained usable: HTTP %d", redirect.StatusCode)
	}
}
