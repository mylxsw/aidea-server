package externalpay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

var jwtEncoding = base64.RawURLEncoding

type claims struct {
	Issuer          string `json:"iss"`
	Audience        string `json:"aud"`
	IssuedAt        int64  `json:"iat"`
	NotBefore       int64  `json:"nbf"`
	ExpiresAt       int64  `json:"exp"`
	ID              string `json:"jti"`
	Method          string `json:"method,omitempty"`
	Path            string `json:"path,omitempty"`
	BodySHA256      string `json:"body_sha256,omitempty"`
	EventID         string `json:"event_id,omitempty"`
	PaymentID       string `json:"payment_id,omitempty"`
	MerchantOrderID string `json:"merchant_order_id,omitempty"`
	Channel         string `json:"channel,omitempty"`
	AmountFen       int64  `json:"amount_fen,omitempty"`
	Currency        string `json:"currency,omitempty"`
	Status          string `json:"status,omitempty"`
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM private key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("expected Ed25519 private key")
	}
	return key, nil
}

func parsePublicKey(value string) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, errors.New("invalid PEM public key")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("expected Ed25519 public key")
	}
	return key, nil
}

func publicKeyPEM(private ed25519.PrivateKey) string {
	der, _ := x509.MarshalPKIXPublicKey(private.Public())
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func signJWT(private ed25519.PrivateKey, payload claims) (string, error) {
	header := []byte(`{"alg":"EdDSA","typ":"JWT"}`)
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	message := jwtEncoding.EncodeToString(header) + "." + jwtEncoding.EncodeToString(body)
	return message + "." + jwtEncoding.EncodeToString(ed25519.Sign(private, []byte(message))), nil
}

func verifyJWT(token string, public ed25519.PublicKey, audience string, now time.Time) (claims, error) {
	var payload claims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return payload, errors.New("invalid JWT")
	}
	header, err := jwtEncoding.DecodeString(parts[0])
	if err != nil {
		return payload, err
	}
	var meta struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(header, &meta); err != nil || meta.Alg != "EdDSA" || meta.Typ != "JWT" {
		return payload, errors.New("unsupported JWT header")
	}
	signature, err := jwtEncoding.DecodeString(parts[2])
	if err != nil {
		return payload, err
	}
	if !ed25519.Verify(public, []byte(parts[0]+"."+parts[1]), signature) {
		return payload, errors.New("invalid JWT signature")
	}
	body, err := jwtEncoding.DecodeString(parts[1])
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return payload, err
	}
	unix := now.Unix()
	if payload.Audience != audience || payload.ID == "" || payload.Issuer == "" || payload.IssuedAt > unix+30 || payload.NotBefore > unix+30 || payload.ExpiresAt < unix || payload.ExpiresAt-payload.IssuedAt > 120 {
		return payload, errors.New("invalid JWT claims")
	}
	return payload, nil
}

func randomID(prefix string) (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(data[:]), nil
}

func randomPaymentID() (string, error) {
	var data [14]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return "EX_" + hex.EncodeToString(data[:]), nil
}

func bodyDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return jwtEncoding.EncodeToString(sum[:])
}

func validateCallbackURL(raw string) (*url.URL, error) {
	if len(raw) > 2048 {
		return nil, errors.New("callback URL too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return nil, errors.New("callback must be a public HTTPS URL on port 443")
	}
	if net.ParseIP(u.Hostname()) != nil || !strings.Contains(u.Hostname(), ".") || strings.HasSuffix(strings.ToLower(u.Hostname()), ".local") {
		return nil, errors.New("callback must use a public DNS name")
	}
	return u, nil
}

var blockedNetworks = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001:db8::/32"),
}

func publicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range blockedNetworks {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

// webhookClient checks the actual DNS result at connection time and pins the dialed IP.
func webhookClient() *http.Client {
	transport := &http.Transport{
		Proxy:               nil,
		MaxIdleConnsPerHost: 1,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, errors.New("callback DNS has no addresses")
			}
			for _, ip := range ips {
				if !publicIP(ip) {
					return nil, errors.New("callback DNS includes a non-public address")
				}
			}
			dialer := &net.Dialer{Timeout: 3 * time.Second}
			return dialer.DialContext(ctx, "tcp", net.JoinHostPort(ips[0].String(), port))
		},
	}
	return &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func postWebhook(ctx context.Context, client *http.Client, callbackURL, jwt string) error {
	if _, err := validateCallbackURL(callbackURL); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, strings.NewReader(jwt))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/jwt")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook HTTP %d", resp.StatusCode)
	}
	return nil
}
