package externalpay

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-pay/gopay"
	ali "github.com/go-pay/gopay/alipay"
	"github.com/mylxsw/aidea-server/config"
)

func alipayTestSigner(t *testing.T) (*gateway, func(url.Values) url.Values) {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&private.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPath := filepath.Join(t.TempDir(), "alipay-public.pem")
	if err := os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	g := &gateway{conf: &config.Config{AliPayAppID: "expected-app", AliPayPublicKeyPath: publicPath}, ali: new(ali.Client)}
	signForm := func(values url.Values) url.Values {
		t.Helper()
		form := make(url.Values)
		params := make(gopay.BodyMap)
		for k, v := range values {
			form.Set(k, v[0])
			params[k] = v[0]
		}
		signData := params.EncodeAliPaySignParams()
		hash := sha256.Sum256([]byte(signData))
		signature, err := rsa.SignPKCS1v15(rand.Reader, private, crypto.SHA256, hash[:])
		if err != nil {
			t.Fatal(err)
		}
		form.Set("sign_type", "RSA2")
		form.Set("sign", base64.StdEncoding.EncodeToString(signature))
		return form
	}
	return g, signForm
}

func TestAlipayCallbackVerifiesSignatureAppAndAmount(t *testing.T) {
	g, signForm := alipayTestSigner(t)
	parse := func(form url.Values) (verifiedPayment, error) {
		r := httptest.NewRequest(http.MethodPost, "/partner/v1/payments/callback/alipay", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return g.alipayNotification(r)
	}
	base := url.Values{"app_id": {"expected-app"}, "out_trade_no": {"EX_test"}, "trade_no": {"alipay-trade"}, "total_amount": {"19.90"}, "trade_status": {"TRADE_SUCCESS"}}
	valid := signForm(base)
	verified, err := parse(valid)
	if err != nil || verified.PaymentID != "EX_test" || verified.AmountFen != 1990 || verified.TradeID != "alipay-trade" {
		t.Fatalf("valid signed callback rejected: %+v, %v", verified, err)
	}
	if !json.Valid(verified.Payload) {
		t.Fatal("signed callback payload was not stored as JSON")
	}
	tampered := signForm(base)
	tampered.Set("total_amount", "20.90")
	if _, err := parse(tampered); err == nil {
		t.Fatal("tampered payment amount accepted")
	}
	wrongApp := url.Values{"app_id": {"another-app"}, "out_trade_no": {"EX_test"}, "trade_no": {"alipay-trade"}, "total_amount": {"19.90"}, "trade_status": {"TRADE_SUCCESS"}}
	if _, err := parse(signForm(wrongApp)); err == nil {
		t.Fatal("signed callback for a different app accepted")
	}
	duplicate := signForm(base)
	duplicate.Add("total_amount", "19.90")
	if _, err := parse(duplicate); err == nil {
		t.Fatal("duplicate payment amount accepted")
	}
}

func TestAlipaySignedCallbackToOrderEndToEnd(t *testing.T) {
	db := externalPayTestDB(t)
	ctx := context.Background()
	var err error
	g, signForm := alipayTestSigner(t)
	service := &Service{store: &Store{db: db}, conf: &config.Config{ExternalPayEnabled: true}, gateway: g}
	api := httptest.NewServer(service.Handler())
	defer api.Close()
	create := func() string {
		t.Helper()
		id, err := randomPaymentID()
		if err != nil {
			t.Fatal(err)
		}
		o := Order{PaymentID: id, ClientID: "alipay-test-app", MerchantOrderID: id, Channel: "alipay_page", AmountFen: 1990, Currency: "CNY", Subject: "test", CallbackURL: "https://callback.example.com/notify"}
		if err := service.store.insertOrder(ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := service.store.prepayReady(ctx, id, "https://alipay.example.com/pay"); err != nil {
			t.Fatal(err)
		}
		return id
	}
	post := func(form url.Values) int {
		t.Helper()
		res, err := api.Client().PostForm(api.URL+"/partner/v1/payments/callback/alipay", form)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	paidID := create()
	tradeID, err := randomID("ali_")
	if err != nil {
		t.Fatal(err)
	}
	form := signForm(url.Values{"app_id": {"expected-app"}, "out_trade_no": {paidID}, "trade_no": {tradeID}, "total_amount": {"19.90"}, "trade_status": {"TRADE_SUCCESS"}})
	if got := post(form); got != 200 {
		t.Fatalf("signed Alipay callback HTTP %d", got)
	}
	if got := post(form); got != 200 {
		t.Fatalf("duplicate Alipay callback HTTP %d", got)
	}
	order, err := service.store.order(ctx, paidID)
	if err != nil || order.Status != "paid" || order.ProviderTradeID != tradeID {
		t.Fatalf("signed Alipay callback did not mark order paid: %+v, %v", order, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_pay_events WHERE payment_id=?`, paidID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate Alipay payment event: %d, %v", count, err)
	}
	unpaidID := create()
	wrongAmount := signForm(url.Values{"app_id": {"expected-app"}, "out_trade_no": {unpaidID}, "trade_no": {"wrong-amount"}, "total_amount": {"19.91"}, "trade_status": {"TRADE_SUCCESS"}})
	if got := post(wrongAmount); got != 400 {
		t.Fatalf("signed wrong Alipay amount HTTP %d", got)
	}
	order, err = service.store.order(ctx, unpaidID)
	if err != nil || order.Status != "pending" {
		t.Fatalf("wrong amount changed order: %+v, %v", order, err)
	}
}

func TestAlipayPrepayUsesExternalNotifyURL(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(dir, "app-private.pem")
	if err := os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0600); err != nil {
		t.Fatal(err)
	}
	certTemplate := &x509.Certificate{SerialNumber: big.NewInt(1234), Subject: pkix.Name{CommonName: "Test Alipay"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), SignatureAlgorithm: x509.SHA256WithRSA}
	certDER, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &private.PublicKey, private)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "app-cert.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0600); err != nil {
		t.Fatal(err)
	}
	conf := &config.Config{EnableAlipay: true, AlipaySandbox: true, ExternalPayBaseURL: "https://pay.example.com", AliPayAppID: "expected-app", AliPayAppPrivateKeyPath: privatePath, AliPayAppPublicKeyPath: certPath, AliPayRootCertPath: certPath, AliPayPublicKeyPath: certPath}
	g, err := newGateway(conf, nil)
	if err != nil {
		t.Fatal(err)
	}
	payURL, err := g.prepay(context.Background(), Order{PaymentID: "EX_test", Channel: "alipay_page", AmountFen: 1990, Subject: "test item"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(payURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("notify_url") != "https://pay.example.com/partner/v1/payments/callback/alipay" || parsed.Query().Get("return_url") != "https://pay.example.com/partner/v1/payments/return" {
		t.Fatalf("Alipay prepay did not use external routes: %s", payURL)
	}
	var content struct {
		PaymentID string `json:"out_trade_no"`
		Amount    string `json:"total_amount"`
	}
	if err := json.Unmarshal([]byte(parsed.Query().Get("biz_content")), &content); err != nil || content.PaymentID != "EX_test" || content.Amount != "19.90" {
		t.Fatalf("incorrect Alipay order content: %+v, %v", content, err)
	}
}
