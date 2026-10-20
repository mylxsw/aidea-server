package externalpay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-pay/gopay"
	ali "github.com/go-pay/gopay/alipay"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/payment/wechatpay"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

type gateway struct {
	conf *config.Config
	ali  *ali.Client
	wx   wechatpay.WeChatPay
}

func newGateway(conf *config.Config, wx wechatpay.WeChatPay) (*gateway, error) {
	g := &gateway{conf: conf, wx: wx}
	if !conf.EnableAlipay {
		return g, nil
	}
	private, err := os.ReadFile(conf.AliPayAppPrivateKeyPath)
	if err != nil {
		return nil, err
	}
	client, err := ali.NewClient(conf.AliPayAppID, string(private), !conf.AlipaySandbox)
	if err != nil {
		return nil, err
	}
	client.SetLocation(ali.LocationShanghai).SetCharset(ali.UTF8).SetSignType(ali.RSA2).
		SetNotifyUrl(strings.TrimRight(conf.ExternalPayBaseURL, "/") + "/partner/v1/payments/callback/alipay").
		SetReturnUrl(strings.TrimRight(conf.ExternalPayBaseURL, "/") + "/partner/v1/payments/return")
	public, err := os.ReadFile(conf.AliPayPublicKeyPath)
	if err != nil {
		return nil, err
	}
	client.AutoVerifySign(public)
	if err := client.SetCertSnByPath(conf.AliPayAppPublicKeyPath, conf.AliPayRootCertPath, conf.AliPayPublicKeyPath); err != nil {
		return nil, err
	}
	g.ali = client
	return g, nil
}

func (g *gateway) prepay(ctx context.Context, o Order) (string, error) {
	switch o.Channel {
	case "alipay_page":
		if g.ali == nil {
			return "", errors.New("alipay unavailable")
		}
		expires := time.Now().Add(15 * time.Minute).In(time.FixedZone("CST", 8*60*60))
		return g.ali.TradePagePay(ctx, gopay.BodyMap{"out_trade_no": o.PaymentID, "total_amount": fmt.Sprintf("%d.%02d", o.AmountFen/100, o.AmountFen%100), "subject": o.Subject, "time_expire": expires.Format("2006-01-02 15:04:05")})
	case "wechat_native":
		if !g.conf.WeChatPayEnabled {
			return "", errors.New("wechat pay unavailable")
		}
		res, err := g.wx.NativePrepay(ctx, wechatpay.PrepayRequest{OutTradeNo: o.PaymentID, Description: o.Subject, Amount: o.AmountFen, NotifyURL: strings.TrimRight(g.conf.ExternalPayBaseURL, "/") + "/partner/v1/payments/callback/wechat"})
		if err != nil {
			return "", err
		}
		return res.CodeURL, nil
	default:
		return "", errors.New("unsupported channel")
	}
}

func parseFen(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return 0, errors.New("invalid amount")
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || yuan < 0 || yuan > 100000000 {
		return 0, errors.New("invalid amount")
	}
	fraction := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, errors.New("invalid amount")
		}
		f, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || f < 0 {
			return 0, errors.New("invalid amount")
		}
		if len(parts[1]) == 1 {
			f *= 10
		}
		fraction = f
	}
	return yuan*100 + fraction, nil
}

type verifiedPayment struct {
	PaymentID, Channel, TradeID string
	AmountFen                   int64
	PaidAt                      time.Time
	Payload                     []byte
	Status                      string
}

func (g *gateway) alipayNotification(r *http.Request) (verifiedPayment, error) {
	var p verifiedPayment
	if g.ali == nil {
		return p, errors.New("alipay unavailable")
	}
	if err := r.ParseForm(); err != nil {
		return p, err
	}
	params := make(map[string]interface{}, len(r.PostForm))
	for k, values := range r.PostForm {
		if len(values) != 1 {
			return p, errors.New("duplicate alipay field")
		}
		params[k] = values[0]
	}
	ok, err := ali.VerifySignWithCert(g.conf.AliPayPublicKeyPath, params)
	if err != nil || !ok {
		return p, errors.New("invalid alipay signature")
	}
	if r.PostForm.Get("app_id") != g.conf.AliPayAppID {
		return p, errors.New("wrong alipay app")
	}
	amount, err := parseFen(r.PostForm.Get("total_amount"))
	if err != nil {
		return p, err
	}
	p = verifiedPayment{PaymentID: r.PostForm.Get("out_trade_no"), Channel: "alipay_page", TradeID: r.PostForm.Get("trade_no"), AmountFen: amount, Status: r.PostForm.Get("trade_status")}
	if p.PaymentID == "" || p.TradeID == "" {
		return p, errors.New("missing alipay transaction identifiers")
	}
	p.Payload, err = json.Marshal(r.PostForm)
	if err != nil {
		return verifiedPayment{}, err
	}
	return p, nil
}

func (g *gateway) wechatNotification(ctx context.Context, r *http.Request) (verifiedPayment, error) {
	var p verifiedPayment
	if !g.conf.WeChatPayEnabled {
		return p, errors.New("wechat pay unavailable")
	}
	private, err := utils.LoadPrivateKeyWithPath(g.conf.WeChatPayCertPrivateKeyPath)
	if err != nil {
		return p, err
	}
	if err := downloader.MgrInstance().RegisterDownloaderWithPrivateKey(ctx, private, g.conf.WeChatPayCertSerialNumber, g.conf.WeChatPayMchID, g.conf.WeChatPayAPIv3Key); err != nil {
		return p, err
	}
	visitor := downloader.MgrInstance().GetCertificateVisitor(g.conf.WeChatPayMchID)
	handler, err := notify.NewRSANotifyHandler(g.conf.WeChatPayAPIv3Key, verifiers.NewSHA256WithRSAVerifier(visitor))
	if err != nil {
		return p, err
	}
	return g.parseWechatNotification(ctx, r, handler)
}

func (g *gateway) parseWechatNotification(ctx context.Context, r *http.Request, handler *notify.Handler) (verifiedPayment, error) {
	var p verifiedPayment
	transaction := new(payments.Transaction)
	notice, err := handler.ParseNotifyRequest(ctx, r, transaction)
	if err != nil {
		return p, err
	}
	if notice.EventType != "TRANSACTION.SUCCESS" {
		return p, errors.New("not a successful transaction")
	}
	return g.validWechatTransaction(transaction, []byte(notice.Resource.Plaintext))
}

func (g *gateway) validWechatTransaction(t *payments.Transaction, payload []byte) (verifiedPayment, error) {
	var p verifiedPayment
	if t == nil || t.Appid == nil || t.Mchid == nil || t.OutTradeNo == nil || t.TransactionId == nil || t.Amount == nil || t.Amount.Total == nil || t.TradeState == nil {
		return p, errors.New("incomplete wechat transaction")
	}
	if *t.Appid != g.conf.WeChatAppID || *t.Mchid != g.conf.WeChatPayMchID {
		return p, errors.New("wrong wechat merchant")
	}
	if t.Amount.Currency != nil && *t.Amount.Currency != "CNY" {
		return p, errors.New("wrong currency")
	}
	if *t.TradeState != "SUCCESS" {
		return p, errors.New("wechat transaction not successful")
	}
	p = verifiedPayment{PaymentID: *t.OutTradeNo, Channel: "wechat_native", TradeID: *t.TransactionId, AmountFen: *t.Amount.Total, Status: *t.TradeState, Payload: payload}
	if t.SuccessTime != nil {
		p.PaidAt, _ = time.Parse(time.RFC3339, *t.SuccessTime)
	}
	return p, nil
}

func (g *gateway) query(ctx context.Context, o Order) (verifiedPayment, error) {
	var p verifiedPayment
	switch o.Channel {
	case "alipay_page":
		if g.ali == nil {
			return p, errors.New("alipay unavailable")
		}
		resp, err := g.ali.TradeQuery(ctx, gopay.BodyMap{"out_trade_no": o.PaymentID})
		if err != nil {
			if business, ok := ali.IsBizError(err); ok && business.SubCode == "ACQ.TRADE_NOT_EXIST" {
				return verifiedPayment{PaymentID: o.PaymentID, Channel: o.Channel, Status: "NOT_EXIST"}, nil
			}
			return p, err
		}
		if resp.Response.OutTradeNo != o.PaymentID {
			return p, errors.New("wrong alipay query order")
		}
		amount, err := parseFen(resp.Response.TotalAmount)
		if err != nil {
			return p, err
		}
		return verifiedPayment{PaymentID: o.PaymentID, Channel: o.Channel, TradeID: resp.Response.TradeNo, AmountFen: amount, Status: resp.Response.TradeStatus}, nil
	case "wechat_native":
		if !g.conf.WeChatPayEnabled {
			return p, errors.New("wechat pay unavailable")
		}
		t, err := g.wx.QueryNative(ctx, o.PaymentID)
		if err != nil {
			return p, err
		}
		if t == nil || t.OutTradeNo == nil || t.Appid == nil || t.Mchid == nil || t.Amount == nil || t.Amount.Total == nil || t.TradeState == nil {
			return p, errors.New("incomplete wechat query")
		}
		if *t.OutTradeNo != o.PaymentID || *t.Appid != g.conf.WeChatAppID || *t.Mchid != g.conf.WeChatPayMchID || *t.Amount.Total != o.AmountFen {
			return p, errors.New("wechat query order mismatch")
		}
		if *t.TradeState != "SUCCESS" {
			return verifiedPayment{PaymentID: o.PaymentID, Channel: o.Channel, Status: *t.TradeState}, nil
		}
		return g.validWechatTransaction(t, nil)
	default:
		return p, errors.New("unsupported channel")
	}
}

func (g *gateway) close(ctx context.Context, o Order) error {
	switch o.Channel {
	case "alipay_page":
		if g.ali == nil {
			return errors.New("alipay unavailable")
		}
		_, err := g.ali.TradeClose(ctx, gopay.BodyMap{"out_trade_no": o.PaymentID})
		return err
	case "wechat_native":
		if !g.conf.WeChatPayEnabled {
			return errors.New("wechat pay unavailable")
		}
		return g.wx.CloseNative(ctx, o.PaymentID)
	default:
		return errors.New("unsupported channel")
	}
}
