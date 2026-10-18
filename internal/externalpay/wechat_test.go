package externalpay

import (
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/payment/wechatpay"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

func signedWechatTestFixture(t *testing.T) (*gateway, *notify.Handler, func(string, string, string, string, int64) *http.Request) {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: big.NewInt(123456), Subject: pkix.Name{CommonName: "Test WeChat Pay"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}, &x509.Certificate{SerialNumber: big.NewInt(123456), Subject: pkix.Name{CommonName: "Test WeChat Pay"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}, &private.PublicKey, private)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatal(err)
	}
	const apiKey = "0123456789abcdef0123456789abcdef"
	block, err := aes.NewCipher([]byte(apiKey))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := notify.NewRSANotifyHandler(apiKey, verifiers.NewSHA256WithRSAVerifier(core.NewCertificateMapWithList([]*x509.Certificate{certificate})))
	if err != nil {
		t.Fatal(err)
	}
	g := &gateway{conf: &config.Config{WeChatAppID: "expected-app", WeChatPayMchID: "expected-mch"}}
	build := func(appID, merchantID, paymentID, tradeID string, amount int64) *http.Request {
		t.Helper()
		transaction, err := json.Marshal(map[string]any{"appid": appID, "mchid": merchantID, "out_trade_no": paymentID, "transaction_id": tradeID, "trade_state": "SUCCESS", "amount": map[string]any{"total": amount, "currency": "CNY"}})
		if err != nil {
			t.Fatal(err)
		}
		const resourceNonce = "123456789012"
		ciphertext := aead.Seal(nil, []byte(resourceNonce), transaction, []byte("transaction"))
		body, err := json.Marshal(map[string]any{"id": "notify-1", "event_type": "TRANSACTION.SUCCESS", "resource": map[string]any{"algorithm": "AEAD_AES_256_GCM", "ciphertext": base64.StdEncoding.EncodeToString(ciphertext), "associated_data": "transaction", "nonce": resourceNonce}})
		if err != nil {
			t.Fatal(err)
		}
		timestamp := time.Now().Unix()
		const headerNonce = "header-nonce"
		signed := []byte(fmt.Sprintf("%d\n%s\n%s\n", timestamp, headerNonce, body))
		hash := sha256.Sum256(signed)
		signature, err := rsa.SignPKCS1v15(rand.Reader, private, crypto.SHA256, hash[:])
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/partner/v1/payments/callback/wechat", strings.NewReader(string(body)))
		r.Header.Set("Wechatpay-Serial", utils.GetCertificateSerialNumber(*certificate))
		r.Header.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(signature))
		r.Header.Set("Wechatpay-Timestamp", strconv.FormatInt(timestamp, 10))
		r.Header.Set("Wechatpay-Nonce", headerNonce)
		return r
	}
	return g, handler, build
}

func TestWechatCallbackVerifiesSignatureDecryptsAndChecksMerchant(t *testing.T) {
	g, handler, build := signedWechatTestFixture(t)
	valid := build("expected-app", "expected-mch", "EX_test", "wx-trade", 1990)
	verified, err := g.parseWechatNotification(context.Background(), valid, handler)
	if err != nil || verified.PaymentID != "EX_test" || verified.TradeID != "wx-trade" || verified.AmountFen != 1990 {
		t.Fatalf("valid WeChat callback rejected: %+v, %v", verified, err)
	}
	wrongMerchant := build("expected-app", "other-mch", "EX_test", "wx-trade", 1990)
	if _, err := g.parseWechatNotification(context.Background(), wrongMerchant, handler); err == nil {
		t.Fatal("signed callback for another merchant accepted")
	}
	tampered := build("expected-app", "expected-mch", "EX_test", "wx-trade", 1990)
	tampered.Header.Set("Wechatpay-Signature", "invalid-signature")
	if _, err := g.parseWechatNotification(context.Background(), tampered, handler); err == nil {
		t.Fatal("invalid WeChat signature accepted")
	}
}

type signedWechatGateway struct {
	*gateway
	handler *notify.Handler
}

func (g signedWechatGateway) wechatNotification(ctx context.Context, r *http.Request) (verifiedPayment, error) {
	return g.gateway.parseWechatNotification(ctx, r, g.handler)
}

func TestWechatSignedCallbackToOrderEndToEnd(t *testing.T) {
	db := externalPayTestDB(t)
	ctx := context.Background()
	var err error
	g, handler, build := signedWechatTestFixture(t)
	service := &Service{store: &Store{db: db}, conf: &config.Config{ExternalPayEnabled: true}, gateway: signedWechatGateway{gateway: g, handler: handler}}
	api := httptest.NewServer(service.Handler())
	defer api.Close()
	create := func() string {
		t.Helper()
		id, err := randomPaymentID()
		if err != nil {
			t.Fatal(err)
		}
		o := Order{PaymentID: id, ClientID: "wechat-test-app", MerchantOrderID: id, Channel: "wechat_native", AmountFen: 1990, Currency: "CNY", Subject: "test", CallbackURL: "https://callback.example.com/notify"}
		if err := service.store.insertOrder(ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := service.store.prepayReady(ctx, id, "weixin://wxpay/test"); err != nil {
			t.Fatal(err)
		}
		return id
	}
	post := func(r *http.Request) int {
		t.Helper()
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(api.URL, "http://")
		r.RequestURI = ""
		res, err := api.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	paidID := create()
	tradeID, err := randomID("wx_")
	if err != nil {
		t.Fatal(err)
	}
	if got := post(build("expected-app", "expected-mch", paidID, tradeID, 1990)); got != 200 {
		t.Fatalf("signed WeChat callback HTTP %d", got)
	}
	if got := post(build("expected-app", "expected-mch", paidID, tradeID, 1990)); got != 200 {
		t.Fatalf("duplicate WeChat callback HTTP %d", got)
	}
	order, err := service.store.order(ctx, paidID)
	if err != nil || order.Status != "paid" || order.ProviderTradeID != tradeID {
		t.Fatalf("signed WeChat callback did not mark order paid: %+v, %v", order, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_pay_events WHERE payment_id=?`, paidID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate WeChat payment event: %d, %v", count, err)
	}
	unpaidID := create()
	if got := post(build("expected-app", "expected-mch", unpaidID, "wrong-amount", 1991)); got != 400 {
		t.Fatalf("signed wrong WeChat amount HTTP %d", got)
	}
	order, err = service.store.order(ctx, unpaidID)
	if err != nil || order.Status != "pending" {
		t.Fatalf("wrong amount changed order: %+v, %v", order, err)
	}
}

type wechatPrepaySpy struct {
	wechatpay.WeChatPayFake
	request wechatpay.PrepayRequest
}

func (s *wechatPrepaySpy) NativePrepay(_ context.Context, request wechatpay.PrepayRequest) (*wechatpay.PrepayResponse, error) {
	s.request = request
	return &wechatpay.PrepayResponse{CodeURL: "weixin://wxpay/test"}, nil
}

func TestWechatPrepayUsesExternalNotifyURL(t *testing.T) {
	spy := &wechatPrepaySpy{}
	g := &gateway{conf: &config.Config{WeChatPayEnabled: true, ExternalPayBaseURL: "https://pay.example.com"}, wx: spy}
	paymentURL, err := g.prepay(context.Background(), Order{PaymentID: "EX_test", Channel: "wechat_native", AmountFen: 1990, Subject: "test item"})
	if err != nil || paymentURL != "weixin://wxpay/test" {
		t.Fatalf("WeChat prepay failed: %s, %v", paymentURL, err)
	}
	if spy.request.OutTradeNo != "EX_test" || spy.request.Amount != 1990 || spy.request.NotifyURL != "https://pay.example.com/partner/v1/payments/callback/wechat" {
		t.Fatalf("WeChat prepay did not use external callback: %+v", spy.request)
	}
}
