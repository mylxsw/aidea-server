package sms

import (
	"context"
	"math/rand"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/aliyun"
	"github.com/mylxsw/aidea-server/internal/tencent"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/glacier/infra"
	"github.com/mylxsw/go-utils/array"
)

type Client struct {
	conf    *config.Config `autowire:"@"`
	tencent *tencent.Tencent
	aliyun  *aliyun.Aliyun
}

func New(resolver infra.Resolver) *Client {
	client := &Client{}
	resolver.MustAutoWire(client)

	if array.In("tencent", client.conf.SMSChannels) {
		resolver.MustResolve(func(c *tencent.Tencent) {
			client.tencent = c
		})
	}

	if array.In("aliyun", client.conf.SMSChannels) {
		resolver.MustResolve(func(c *aliyun.Aliyun) {
			client.aliyun = c
		})
	}

	return client
}

func (client *Client) SendVerifyCode(ctx context.Context, verifyCode string, receiver string) error {
	log.Debugf("send sms to %s, verify code %s", receiver, verifyCode)

	selectedClient := client.conf.SMSChannels[rand.Intn(len(client.conf.SMSChannels))]
	switch selectedClient {
	case "tencent":
		return client.tencent.SendSMS(ctx, "1822196", []string{verifyCode}, receiver)
	case "aliyun":
		return client.aliyun.SendSMS(ctx, "SMS_279297328", map[string]string{"code": verifyCode}, receiver)
	default:
		log.Errorf("invalid sms client selected: %s", selectedClient)
	}

	return nil
}
