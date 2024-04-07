package wechatpay

import (
	"context"
	"github.com/pkg/errors"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/app"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

type Config struct {
	WeChatAppID                 string `json:"wechat_appid" yaml:"wechat_appid"`
	WeChatPayMchID              string `json:"wechat_pay_mchid" yaml:"wechat_pay_mchid"`
	WeChatPayCertSerialNumber   string `json:"wechat_pay_cert_serial_number" yaml:"wechat_pay_cert_serial_number"`
	WeChatPayCertPrivateKeyPath string `json:"wechat_pay_cert_private_key_path" yaml:"wechat_pay_cert_private_key_path"`
	WeChatPayAPIv3Key           string `json:"wechat_pay_apiv3_key" yaml:"wechat_pay_apiv3_key"`
}

type WeChatPay struct {
	conf *Config
}

func NewWeChatPay(conf *Config) *WeChatPay {
	return &WeChatPay{conf: conf}
}

type PrepayRequest struct {
	OutTradeNo  string
	Description string
	NotifyURL   string
	Amount      int64
}

type PrepayResponse struct {
	CodeURL  string `json:"code_url,omitempty"`
	PrepayID string `json:"prepay_id,omitempty"`
}

// NativePrepay creates a native prepay order
func (w *WeChatPay) NativePrepay(ctx context.Context, req PrepayRequest) (*PrepayResponse, error) {
	client, err := w.createClient(ctx)
	if err != nil {
		return nil, err
	}

	svc := native.NativeApiService{Client: client}
	resp, _, err := svc.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(w.conf.WeChatAppID),
		Mchid:       core.String(w.conf.WeChatPayMchID),
		Description: core.String(req.Description),
		OutTradeNo:  core.String(req.OutTradeNo),
		NotifyUrl:   core.String(req.NotifyURL),
		Amount: &native.Amount{
			Total: core.Int64(req.Amount),
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "prepay failed")
	}

	if resp.CodeUrl == nil {
		return nil, errors.New("prepay failed, code url is empty")
	}

	return &PrepayResponse{CodeURL: *resp.CodeUrl}, nil
}

// AppPrepay creates a app prepay order
func (w *WeChatPay) AppPrepay(ctx context.Context, req PrepayRequest) (*PrepayResponse, error) {
	client, err := w.createClient(ctx)
	if err != nil {
		return nil, err
	}

	svc := app.AppApiService{Client: client}
	resp, _, err := svc.Prepay(ctx, app.PrepayRequest{
		Appid:       core.String(w.conf.WeChatAppID),
		Mchid:       core.String(w.conf.WeChatPayMchID),
		Description: core.String(req.Description),
		OutTradeNo:  core.String(req.OutTradeNo),
		NotifyUrl:   core.String(req.NotifyURL),
		Amount: &app.Amount{
			Total: core.Int64(req.Amount),
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "prepay failed")
	}

	if resp.PrepayId == nil {
		return nil, errors.New("prepay failed, prepay id is empty")
	}

	return &PrepayResponse{PrepayID: *resp.PrepayId}, nil
}

// createClient creates a wechat pay client
func (w *WeChatPay) createClient(ctx context.Context) (*core.Client, error) {
	mchPrivateKey, err := utils.LoadPrivateKeyWithPath(w.conf.WeChatPayCertPrivateKeyPath)
	if err != nil {
		return nil, errors.Wrap(err, "load private key failed")
	}

	opts := []core.ClientOption{
		option.WithWechatPayAutoAuthCipher(w.conf.WeChatPayMchID, w.conf.WeChatPayCertSerialNumber, mchPrivateKey, w.conf.WeChatPayAPIv3Key),
	}
	client, err := core.NewClient(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "create wechat pay client failed")
	}

	return client, nil
}
