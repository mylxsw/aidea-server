package controllers_test

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/Timothylock/go-signin-with-apple/apple"
	"github.com/mylxsw/asteria/log"
)

func TestRevokeAppleToken(t *testing.T) {
	client := apple.New()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	data, err := os.ReadFile("/Users/mylxsw/ResilioSync/AI/AuthKey_34G9XW98XP.p8")
	if err != nil {
		panic(err)
	}

	appleSecret := string(data)

	secret, err := apple.GenerateClientSecret(
		appleSecret,
		"N95437SZ2A",
		"cc.aicode.flutter.askaide.askaide",
		"34G9XW98XP",
	)
	if err != nil {
		log.Errorf("generate client secret for revoke apple account failed: %v", err)
	} else {
		req := apple.RevokeAccessTokenRequest{
			ClientID:     "cc.aicode.flutter.askaide.askaide",
			ClientSecret: secret,
			AccessToken:  "001569.7845e22193df4b65b60736e637bee3c8.0834",
		}
		var resp apple.RevokeResponse
		if err := client.RevokeAccessToken(ctx, req, &resp); err != nil {
			if err != io.EOF {
				log.Errorf("revoke apple access token failed: %v", err)
			}
		}

		log.With(resp).Debug("response")
	}
}
