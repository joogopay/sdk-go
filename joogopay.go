package joogopay

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
	merchantrequest "github.com/joogopay/sdk-go/internal/merchantrequest"
)

const (
	defaultTimeout          = 30 * time.Second
	defaultMaxResponseBytes = 8 << 20
	defaultUserAgent        = "merchant-sdk-go"
	defaultAcceptLanguage   = "en-US"
)

type Config struct {
	BaseURL string

	AccessKey                string
	MerchantPrivateKeyBase64 string

	PlatformBodyKeyID           string
	PlatformBodyPublicKeyBase64 string

	PlatformWebhookPublicKeys map[string]string

	HTTPClient *http.Client
	Timeout    time.Duration

	UserAgent        string
	AcceptLanguage   string
	MaxResponseBytes int64
}

type Client struct {
	baseURL string

	accessKey          string
	merchantPrivateKey ed25519.PrivateKey

	platformBodyKeyID     string
	platformBodyPublicKey []byte

	platformWebhookPublicKeys map[string]ed25519.PublicKey

	httpClient       *http.Client
	userAgent        string
	acceptLanguage   string
	maxResponseBytes int64
	now              func() time.Time
}

type Option func(*Client)

// WithClock overrides the clock used for signature timestamps.
func WithClock(now func() time.Time) Option {
	return func(c *Client) {
		if now != nil {
			c.now = now
		}
	}
}

func NewClient(cfg Config, opts ...Option) (*Client, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
		return nil, ErrInvalidBaseURL
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, ErrInvalidBaseURL
	}

	accessKey := strings.TrimSpace(cfg.AccessKey)
	if accessKey == "" {
		return nil, ErrMissingAccessKey
	}
	merchantPrivateKey, err := decodeEd25519PrivateKey(cfg.MerchantPrivateKeyBase64)
	if err != nil {
		return nil, err
	}
	platformBodyKeyID := strings.TrimSpace(cfg.PlatformBodyKeyID)
	if platformBodyKeyID == "" {
		return nil, ErrMissingPlatformBodyKeyID
	}
	if strings.TrimSpace(cfg.PlatformBodyPublicKeyBase64) == "" {
		return nil, ErrMissingPlatformBodyPublicKey
	}
	platformBodyPublicKey, err := decodeFixedKey(cfg.PlatformBodyPublicKeyBase64, merchantauth.X25519PublicKeySize, ErrInvalidPlatformBodyPublicKey)
	if err != nil {
		return nil, err
	}
	platformWebhookKeys, err := decodePlatformWebhookKeys(cfg.PlatformWebhookPublicKeys)
	if err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	maxResponseBytes := cfg.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}

	client := &Client{
		baseURL:                   strings.TrimRight(baseURL, "/"),
		accessKey:                 accessKey,
		merchantPrivateKey:        merchantPrivateKey,
		platformBodyKeyID:         platformBodyKeyID,
		platformBodyPublicKey:     platformBodyPublicKey,
		platformWebhookPublicKeys: platformWebhookKeys,
		httpClient:                httpClient,
		userAgent:                 firstNonEmpty(cfg.UserAgent, defaultUserAgent),
		acceptLanguage:            firstNonEmpty(cfg.AcceptLanguage, defaultAcceptLanguage),
		maxResponseBytes:          maxResponseBytes,
		now:                       time.Now,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client, nil
}

// decodeEd25519PrivateKey accepts both a 32-byte seed and a 64-byte private key:
// libsodium and OpenSSL hand merchants the seed, Go's ed25519.PrivateKey is seed
// plus public key, and accepting only the latter locks merchants out with a valid key.
func decodeEd25519PrivateKey(value string) (ed25519.PrivateKey, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, ErrMissingMerchantPrivateKey
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidMerchantPrivateKey
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, ErrInvalidMerchantPrivateKey
	}
}

func decodePlatformWebhookKeys(values map[string]string) (map[string]ed25519.PublicKey, error) {
	if len(values) == 0 {
		return nil, ErrMissingPlatformWebhookPublicKeys
	}
	out := make(map[string]ed25519.PublicKey, len(values))
	for keyID, value := range values {
		keyID = strings.TrimSpace(keyID)
		if keyID == "" {
			return nil, ErrInvalidPlatformWebhookPublicKey
		}
		publicKey, err := decodeFixedKey(value, ed25519.PublicKeySize, ErrInvalidPlatformWebhookPublicKey)
		if err != nil {
			return nil, err
		}
		out[keyID] = ed25519.PublicKey(publicKey)
	}
	return out, nil
}

func decodeFixedKey(value string, size int, sentinel error) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(raw) != size {
		return nil, sentinel
	}
	return raw, nil
}

func (c *Client) merchantSigner() merchantrequest.Ed25519Signer {
	return merchantrequest.Ed25519Signer{PrivateKey: c.merchantPrivateKey}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if s := strings.TrimSpace(value); s != "" {
			return s
		}
	}
	return ""
}

type envelope struct {
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
	TraceID string          `json:"traceId"`
	Data    json.RawMessage `json:"data"`
}

func (e envelope) dataMessage() string {
	var d struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(e.Data, &d)
	return d.Message
}
