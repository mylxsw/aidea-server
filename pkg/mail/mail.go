package mail

import (
	"github.com/mylxsw/aidea-server/config"
	"gopkg.in/gomail.v2"
)

type Sender struct {
	conf   config.Mail
	dailer *gomail.Dialer
}

func NewSender(conf config.Mail) *Sender {
	dailer := gomail.NewDialer(conf.SMTPHost, conf.SMTPPort, conf.SMTPUsername, conf.SMTPPassword)
	dailer.SSL = conf.UseSSL

	return &Sender{conf: conf, dailer: dailer}
}

func (m *Sender) Send(to []string, subject, body string) error {
	msg := gomail.NewMessage()
	msg.SetAddressHeader("From", m.conf.SMTPUsername, m.conf.From)
	msg.SetHeader("To", to...)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/plain", body)

	return m.dailer.DialAndSend(msg)
}
