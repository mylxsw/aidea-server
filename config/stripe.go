package config

import "github.com/stripe/stripe-go/v76"

type StripeConfig struct {
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	PublishableKey string `json:"publishable_key" yaml:"publishable_key"`
	SecretKey      string `json:"secret_key" yaml:"secret_key"`
	WebhookSecret  string `json:"webhook_secret" yaml:"webhook_secret"`
}

func (s StripeConfig) Init() {
	stripe.Key = s.SecretKey
}
