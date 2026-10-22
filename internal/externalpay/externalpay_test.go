package externalpay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/payment/wechatpay"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

func TestJWTSignatureRequestBindingAndExpiry(t *testing.T) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Second)
	c := claims{Issuer: "app", Audience: "aidea-external-pay", IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), ID: "unique", Method: "POST", Path: "/partner/v1/payments", BodySHA256: bodyDigest([]byte(`{"amount_fen":100}`))}
	token, err := signJWT(private, c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := verifyJWT(token, pub, "aidea-external-pay", now)
	if err != nil || got != c {
		t.Fatalf("valid JWT rejected: %v", err)
	}
	if _, err := verifyJWT(token, pub, "aidea-external-pay", now.Add(2*time.Minute)); err == nil {
		t.Fatal("expired JWT accepted")
	}
	if _, err := verifyJWT(token, pub, "another-audience", now); err == nil {
		t.Fatal("wrong audience accepted")
	}
	parts := strings.Split(token, ".")
	if parts[1][0] == 'A' {
		parts[1] = "B" + parts[1][1:]
	} else {
		parts[1] = "A" + parts[1][1:]
	}
	if _, err := verifyJWT(strings.Join(parts, "."), pub, "aidea-external-pay", now); err == nil {
		t.Fatal("modified JWT accepted")
	}
	if got.Method != http.MethodPost || got.Path != "/partner/v1/payments" || got.BodySHA256 == bodyDigest([]byte(`{"amount_fen":101}`)) {
		t.Fatal("request binding was lost")
	}
}

func TestDocumentationAvailableWhenPaymentsDisabled(t *testing.T) {
	service := &Service{conf: &config.Config{ExternalPayEnabled: false}}
	docs := httptest.NewRecorder()
	service.Handler().ServeHTTP(docs, httptest.NewRequest(http.MethodGet, "/partner/v1/payments/docs", nil))
	if docs.Code != http.StatusOK || !strings.Contains(docs.Body.String(), "创建支付单") {
		t.Fatalf("documentation unavailable: HTTP %d", docs.Code)
	}
	order := httptest.NewRecorder()
	service.Handler().ServeHTTP(order, httptest.NewRequest(http.MethodPost, "/partner/v1/payments", nil))
	if order.Code != http.StatusNotFound {
		t.Fatalf("payment endpoint available while disabled: HTTP %d", order.Code)
	}
}

func TestPaymentReturnPageDoesNotTreatRedirectAsPaymentSuccess(t *testing.T) {
	service := &Service{conf: &config.Config{ExternalPayEnabled: true}}
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/partner/v1/payments/return?out_trade_no=PAYMENT_MARKER&trade_status=TRADE_SUCCESS", nil))

	if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("return page unavailable: HTTP %d, content type %q", response.Code, response.Header().Get("Content-Type"))
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("return page may expose or cache payment redirect parameters")
	}
	body := response.Body.String()
	if !strings.Contains(body, "回到应用") || !strings.Contains(body, "window.close()") || !strings.Contains(body, "手动关闭") {
		t.Fatal("return page does not provide close and fallback guidance")
	}
	if strings.Contains(body, "PAYMENT_MARKER") || strings.Contains(body, "支付成功") {
		t.Fatal("return page trusts or exposes unverified redirect parameters")
	}

	wrongMethod := httptest.NewRecorder()
	service.Handler().ServeHTTP(wrongMethod, httptest.NewRequest(http.MethodPost, "/partner/v1/payments/return", nil))
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("return page accepted POST: HTTP %d", wrongMethod.Code)
	}
}

func TestAuthenticateBindsRequestAndRejectsReplayedJWT(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	requestBody := []byte(`{"amount_fen":100}`)
	now := time.Now().Unix()
	token, err := signJWT(private, claims{Issuer: "app", Audience: "aidea-external-pay", IssuedAt: now, NotBefore: now, ExpiresAt: now + 60, ID: "nonce-1", Method: "POST", Path: "/partner/v1/payments", BodySHA256: bodyDigest(requestBody)})
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{store: &Store{db: db}}
	newRequest := func(path string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(requestBody)))
		r.Header.Set("Authorization", "Bearer "+token)
		return r
	}
	clientRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"client_id", "public_key_pem", "enabled", "allowed_channels", "min_amount_fen", "max_amount_fen", "max_daily_orders"}).AddRow("app", publicPEM, 1, "wechat_native", 1, 10000, 100)
	}
	mock.ExpectQuery("SELECT client_id, public_key_pem").WithArgs("app").WillReturnRows(clientRows())
	mock.ExpectExec("INSERT INTO external_pay_nonces").WithArgs("app", "nonce-1", now+60).WillReturnResult(sqlmock.NewResult(1, 1))
	if _, err := service.authenticate(newRequest("/partner/v1/payments"), requestBody); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	mock.ExpectQuery("SELECT client_id, public_key_pem").WithArgs("app").WillReturnRows(clientRows())
	mock.ExpectExec("INSERT INTO external_pay_nonces").WithArgs("app", "nonce-1", now+60).WillReturnError(sqlmock.ErrCancelled)
	if _, err := service.authenticate(newRequest("/partner/v1/payments"), requestBody); err == nil {
		t.Fatal("replayed request accepted")
	}
	mock.ExpectQuery("SELECT client_id, public_key_pem").WithArgs("app").WillReturnRows(clientRows())
	if _, err := service.authenticate(newRequest("/partner/v1/payments/other"), requestBody); err == nil {
		t.Fatal("token accepted on another path")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallbackURLAndPaymentID(t *testing.T) {
	for _, raw := range []string{"http://example.com/pay", "https://localhost/pay", "https://127.0.0.1/pay", "https://example.com:8443/pay", "https://user@example.com/pay", "https://example.local/pay"} {
		if _, err := validateCallbackURL(raw); err == nil {
			t.Errorf("accepted unsafe callback %q", raw)
		}
	}
	if _, err := validateCallbackURL("https://payments.example.com/notify"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "::1", "fc00::1"} {
		if publicIP(net.ParseIP(raw)) {
			t.Errorf("accepted private IP %s", raw)
		}
	}
	if !publicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public IP rejected")
	}
	id, err := randomPaymentID()
	if err != nil || !strings.HasPrefix(id, "EX_") || len(id) > 32 {
		t.Fatalf("invalid payment ID %q: %v", id, err)
	}
}

func TestAmountAndWechatMerchantValidation(t *testing.T) {
	for input, want := range map[string]int64{"1": 100, "1.2": 120, "1.20": 120, "0.01": 1} {
		got, err := parseFen(input)
		if err != nil || got != want {
			t.Errorf("parseFen(%q) = %d, %v", input, got, err)
		}
	}
	for _, input := range []string{"1.234", "1e2", "-1", "", "1."} {
		if _, err := parseFen(input); err == nil {
			t.Errorf("accepted invalid amount %q", input)
		}
	}
	g := &gateway{conf: &config.Config{WeChatAppID: "appid", WeChatPayMchID: "mchid"}}
	app, mch, out, trade, status, currency := "appid", "mchid", "EX_123", "wx-trade", "SUCCESS", "CNY"
	amount := int64(100)
	tx := &payments.Transaction{Appid: &app, Mchid: &mch, OutTradeNo: &out, TransactionId: &trade, TradeState: &status, Amount: &payments.TransactionAmount{Total: &amount, Currency: &currency}}
	if _, err := g.validWechatTransaction(tx, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	wrong := "different-merchant"
	tx.Mchid = &wrong
	if _, err := g.validWechatTransaction(tx, nil); err == nil {
		t.Fatal("wrong merchant accepted")
	}
}

func TestDisabledWechatOrdersDoNotPanicDuringReconciliation(t *testing.T) {
	g := &gateway{conf: &config.Config{WeChatPayEnabled: false}, wx: &wechatpay.WeChatPayFake{}}
	if _, err := g.query(context.Background(), Order{PaymentID: "EX_test", Channel: "wechat_native"}); err == nil {
		t.Fatal("disabled channel was queried")
	}
}

func TestMarkPaidCreatesOneEventAndRejectsWrongAmount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &Store{db: db}
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"channel", "amount_fen", "status"}).AddRow("wechat_native", 100, "pending")
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT channel,amount_fen,status FROM external_pay_orders").WithArgs("EX_abc").WillReturnRows(rows())
	mock.ExpectRollback()
	if changed, err := store.markPaid(context.Background(), "EX_abc", "wechat_native", "wx1", 99, []byte(`{}`)); err == nil || changed {
		t.Fatal("wrong amount accepted")
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT channel,amount_fen,status FROM external_pay_orders").WithArgs("EX_abc").WillReturnRows(rows())
	mock.ExpectExec("UPDATE external_pay_orders SET status='paid'").WithArgs("wx1", []byte(`{}`), "EX_abc").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO external_pay_events").WithArgs(sqlmock.AnyArg(), "EX_abc").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if changed, err := store.markPaid(context.Background(), "EX_abc", "wechat_native", "wx1", 100, []byte(`{}`)); err != nil || !changed {
		t.Fatalf("valid payment rejected: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT channel,amount_fen,status FROM external_pay_orders").WithArgs("EX_abc").WillReturnRows(sqlmock.NewRows([]string{"channel", "amount_fen", "status"}).AddRow("wechat_native", 100, "paid"))
	mock.ExpectRollback()
	if changed, err := store.markPaid(context.Background(), "EX_abc", "wechat_native", "wx1", 100, nil); err != nil || changed {
		t.Fatalf("duplicate payment generated event: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
