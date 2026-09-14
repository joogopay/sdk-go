package joogopay

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestNewClientValidation(t *testing.T) {
	k := newTestKeys(t)
	base := k.config("https://api.example.com")

	cases := []struct {
		name string
		mut  func(*Config)
		want error
	}{
		{"missing baseURL", func(c *Config) { c.BaseURL = " " }, ErrMissingBaseURL},
		{"invalid baseURL scheme", func(c *Config) { c.BaseURL = "api.example.com" }, ErrInvalidBaseURL},
		{"http baseURL", func(c *Config) { c.BaseURL = "http://api.example.com" }, ErrInvalidBaseURL},
		{"non-HTTPS baseURL", func(c *Config) { c.BaseURL = "ftp://api.example.com" }, ErrInvalidBaseURL},
		{"baseURL with path", func(c *Config) { c.BaseURL = "https://api.example.com/v1" }, ErrInvalidBaseURL},
		{"baseURL with query", func(c *Config) { c.BaseURL = "https://api.example.com?x=1" }, ErrInvalidBaseURL},
		{"missing accessKey", func(c *Config) { c.AccessKey = "" }, ErrMissingAccessKey},
		{"missing private key", func(c *Config) { c.MerchantPrivateKeyBase64 = "" }, ErrMissingMerchantPrivateKey},
		{"invalid private key", func(c *Config) { c.MerchantPrivateKeyBase64 = "!!" }, ErrInvalidMerchantPrivateKey},
		{"missing body keyID", func(c *Config) { c.PlatformBodyKeyID = "" }, ErrMissingPlatformBodyKeyID},
		{"missing body pubkey", func(c *Config) { c.PlatformBodyPublicKeyBase64 = "" }, ErrMissingPlatformBodyPublicKey},
		{"invalid body pubkey", func(c *Config) { c.PlatformBodyPublicKeyBase64 = "abc" }, ErrInvalidPlatformBodyPublicKey},
		{"missing webhook keys", func(c *Config) { c.PlatformWebhookPublicKeys = nil }, ErrMissingPlatformWebhookPublicKeys},
		{"blank webhook keyid", func(c *Config) {
			c.PlatformWebhookPublicKeys = map[string]string{" ": base64.StdEncoding.EncodeToString(k.webhookPub)}
		}, ErrInvalidPlatformWebhookPublicKey},
		{"invalid webhook pubkey", func(c *Config) { c.PlatformWebhookPublicKeys = map[string]string{"w": "zz"} }, ErrInvalidPlatformWebhookPublicKey},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mut(&cfg)
			if _, err := NewClient(cfg); err != tc.want {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestNewClientDefaultsAndOption(t *testing.T) {
	k := newTestKeys(t)
	called := false
	c, err := NewClient(k.config("https://api.example.com"), WithClock(func() time.Time {
		called = true
		return time.Unix(1787803200, 0)
	}))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_ = c.now()
	if !called {
		t.Fatal("WithClock not applied")
	}
	if c.userAgent != defaultUserAgent || c.acceptLanguage != defaultAcceptLanguage || c.maxResponseBytes != defaultMaxResponseBytes {
		t.Fatal("defaults not applied")
	}
	if c.merchantSigner().PrivateKey == nil {
		t.Fatal("signer missing key")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if firstNonEmpty("", " ", "x") != "x" {
		t.Fatal("pick")
	}
	if firstNonEmpty("", "  ") != "" {
		t.Fatal("empty")
	}
}

func TestEnvelopeDataMessage(t *testing.T) {
	e := envelope{Data: []byte(`{"message":"boom"}`)}
	if e.dataMessage() != "boom" {
		t.Fatal("dataMessage")
	}
	if (envelope{Data: []byte(`not json`)}).dataMessage() != "" {
		t.Fatal("dataMessage on bad json should be empty")
	}
}
