package joogopay

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
)

type bodyCryptVector struct {
	Envelope                     merchantauth.BodyEnvelope `json:"envelope"`
	KeyID                        string                    `json:"keyId"`
	PlaintextBase64              string                    `json:"plaintextBase64"`
	PlatformBodyPrivateKeyBase64 string                    `json:"platformBodyPrivateKeyBase64"`
	PlatformBodyPublicKeyBase64  string                    `json:"platformBodyPublicKeyBase64"`
}

type webhookVector struct {
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
	Input   struct {
		Method   string `json:"method"`
		Path     string `json:"path"`
		RawQuery string `json:"rawQuery"`
	} `json:"input"`
	Expected struct {
		Signature     string `json:"signature"`
		SignatureBase string `json:"signatureBase"`
	} `json:"expected"`
	Key struct {
		PlatformWebhookKeyID        string `json:"platformWebhookKeyId"`
		PlatformWebhookPublicKeyB64 string `json:"platformWebhookPublicKeyBase64"`
	} `json:"key"`
}

type emptyObjectPostVector struct {
	Plaintext       string                    `json:"plaintext"`
	Envelope        merchantauth.BodyEnvelope `json:"envelope"`
	PlatformBodyKey struct {
		KeyID            string `json:"keyId"`
		PublicKeyBase64  string `json:"publicKeyBase64"`
		PrivateKeyBase64 string `json:"privateKeyBase64"`
	} `json:"platformBodyKey"`
	Input struct {
		Method         string `json:"method"`
		Path           string `json:"path"`
		RawQuery       string `json:"rawQuery"`
		AccessKey      string `json:"accessKey"`
		IdempotencyKey string `json:"idempotencyKey"`
		Nonce          string `json:"nonce"`
	} `json:"input"`
	MerchantKey struct {
		PrivateKeyBase64 string `json:"merchantPrivateKeyBase64"`
		PublicKeyBase64  string `json:"merchantPublicKeyBase64"`
	} `json:"merchantKey"`
	Expected struct {
		ContentDigest  string `json:"contentDigest"`
		SignatureInput string `json:"signatureInput"`
		SignatureBase  string `json:"signatureBase"`
		Signature      string `json:"signature"`
	} `json:"expected"`
}

func readProtocolVector[T any](t *testing.T, rel string) T {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("protocol", "testdata", rel))
	if err != nil {
		t.Fatalf("read vector %s: %v", rel, err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse vector %s: %v", rel, err)
	}
	return v
}

func TestBodyCryptVector(t *testing.T) {
	v := readProtocolVector[bodyCryptVector](t, "bodycrypt/001-sealed-box.json")

	wire, err := json.Marshal(v.Envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if got := merchantauth.PeekBodyEnvelopeKeyID(wire); got != v.KeyID {
		t.Fatalf("key id mismatch: got %s want %s", got, v.KeyID)
	}

	publicKey := decodeVectorKey(t, v.PlatformBodyPublicKeyBase64)
	privateKey := decodeVectorKey(t, v.PlatformBodyPrivateKeyBase64)
	wantPlain := decodeVectorKey(t, v.PlaintextBase64)

	gotPlain, gotKeyID, err := merchantauth.OpenBodyEnvelope(wire, publicKey, privateKey)
	if err != nil {
		t.Fatalf("open vector envelope: %v", err)
	}
	if gotKeyID != v.KeyID {
		t.Fatalf("opened key id mismatch: got %s want %s", gotKeyID, v.KeyID)
	}
	if !bytes.Equal(gotPlain, wantPlain) {
		t.Fatalf("opened plaintext mismatch:\n got %s\nwant %s", gotPlain, wantPlain)
	}
	wrongKey := bytes.Repeat([]byte{0x44}, merchantauth.X25519PublicKeySize)
	if _, _, err := merchantauth.OpenBodyEnvelope(wire, wrongKey, wrongKey); err == nil {
		t.Fatal("vector envelope must reject the wrong X25519 key pair")
	}

	tampered := v.Envelope
	ciphertext, err := base64.StdEncoding.DecodeString(tampered.Ciphertext)
	if err != nil {
		t.Fatalf("decode vector ciphertext: %v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 0x01
	tampered.Ciphertext = base64.StdEncoding.EncodeToString(ciphertext)
	tamperedWire, err := json.Marshal(tampered)
	if err != nil {
		t.Fatalf("marshal tampered envelope: %v", err)
	}
	if _, _, err := merchantauth.OpenBodyEnvelope(tamperedWire, publicKey, privateKey); err == nil {
		t.Fatal("vector envelope must reject authenticated ciphertext tampering")
	}

	roundTripWire, err := merchantauth.SealBodyEnvelope(wantPlain, publicKey, v.KeyID)
	if err != nil {
		t.Fatalf("seal envelope: %v", err)
	}
	roundTripPlain, _, err := merchantauth.OpenBodyEnvelope(roundTripWire, publicKey, privateKey)
	if err != nil {
		t.Fatalf("open roundtrip envelope: %v", err)
	}
	if !bytes.Equal(roundTripPlain, wantPlain) {
		t.Fatalf("roundtrip plaintext mismatch")
	}
}

func TestEmptyObjectPostVector(t *testing.T) {
	v := readProtocolVector[emptyObjectPostVector](t, "empty-object-post/001-empty-object.json")
	if v.Plaintext != `{}` {
		t.Fatalf("plaintext = %q, want {}", v.Plaintext)
	}
	wire, err := json.Marshal(v.Envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if got := merchantauth.ContentDigestSHA256(wire); got != v.Expected.ContentDigest {
		t.Fatalf("content digest = %s, want %s", got, v.Expected.ContentDigest)
	}
	plain, keyID, err := merchantauth.OpenBodyEnvelope(
		wire,
		decodeVectorKey(t, v.PlatformBodyKey.PublicKeyBase64),
		decodeVectorKey(t, v.PlatformBodyKey.PrivateKeyBase64),
	)
	if err != nil || string(plain) != v.Plaintext || keyID != v.PlatformBodyKey.KeyID {
		t.Fatalf("open empty-object vector: plaintext=%q keyID=%q err=%v", plain, keyID, err)
	}

	r := httptest.NewRequest(v.Input.Method, v.Input.Path, bytes.NewReader(wire))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(merchantauth.HeaderContentEncryption, merchantauth.ContentEncryption)
	r.Header.Set(merchantauth.HeaderContentDigest, v.Expected.ContentDigest)
	r.Header.Set(merchantauth.HeaderIdempotencyKey, v.Input.IdempotencyKey)
	r.Header.Set(merchantauth.HeaderMerchantAccessKey, v.Input.AccessKey)
	params := merchantauth.SignatureParams{
		Label:   merchantauth.SignatureLabelMerchant,
		Covered: merchantauth.MerchantWriteCoveredComponents(),
		Created: 1787803200,
		Expires: 1787803500,
		Nonce:   v.Input.Nonce,
		Alg:     merchantauth.SignatureAlgEd25519,
	}
	signatureInput, err := merchantauth.SignatureInputHeader(params)
	if err != nil || signatureInput != v.Expected.SignatureInput {
		t.Fatalf("signature input=%q err=%v", signatureInput, err)
	}
	r.Header.Set(merchantauth.HeaderSignatureInput, signatureInput)
	base, err := merchantauth.SignatureBase(r, params)
	if err != nil || string(base) != v.Expected.SignatureBase {
		t.Fatalf("signature base=%q err=%v", base, err)
	}
	signature, err := merchantauth.SignEd25519(
		ed25519.PrivateKey(decodeVectorKey(t, v.MerchantKey.PrivateKeyBase64)),
		merchantauth.SignatureLabelMerchant,
		base,
	)
	if err != nil || signature != v.Expected.Signature {
		t.Fatalf("signature=%q err=%v", signature, err)
	}
	if _, err := merchantauth.SealBodyEnvelope(nil, decodeVectorKey(t, v.PlatformBodyKey.PublicKeyBase64), v.PlatformBodyKey.KeyID); err == nil {
		t.Fatal("empty HTTP body must be rejected")
	}
}

func TestWebhookVector(t *testing.T) {
	v := readProtocolVector[webhookVector](t, "webhook/001-payment-succeeded.json")
	sig := readSignatureVector(t, "001-read-query.json")
	bodyKey := readProtocolVector[bodyCryptVector](t, "bodycrypt/001-sealed-box.json")
	now := time.Unix(1787803200, 0)

	c, err := NewClient(Config{
		BaseURL:                     "https://api.example.com",
		AccessKey:                   sig.Input.AccessKey,
		MerchantPrivateKeyBase64:    sig.Key.MerchantPrivateKeyBase64,
		PlatformBodyKeyID:           bodyKey.KeyID,
		PlatformBodyPublicKeyBase64: bodyKey.PlatformBodyPublicKeyBase64,
		PlatformWebhookPublicKeys: map[string]string{
			v.Key.PlatformWebhookKeyID: v.Key.PlatformWebhookPublicKeyB64,
		},
	}, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	r, err := http.NewRequest(v.Input.Method, webhookVectorURL(v), bytes.NewReader([]byte(v.Body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for name, value := range v.Headers {
		r.Header.Set(name, value)
	}

	params, err := merchantauth.ParsePlatformSignatureInput(v.Headers[merchantauth.HeaderSignatureInput])
	if err != nil {
		t.Fatalf("parse signature input: %v", err)
	}
	base, err := merchantauth.SignatureBase(r, params)
	if err != nil {
		t.Fatalf("signature base: %v", err)
	}
	if string(base) != v.Expected.SignatureBase {
		t.Fatalf("signature base mismatch:\n got %q\nwant %q", base, v.Expected.SignatureBase)
	}
	if v.Headers[merchantauth.HeaderSignature] != v.Expected.Signature {
		t.Fatalf("signature header mismatch")
	}
	parsedSignature, err := merchantauth.ParseSignature(
		v.Headers[merchantauth.HeaderSignature],
		merchantauth.SignatureLabelPlatform,
	)
	if err != nil {
		t.Fatalf("parse fixed signature: %v", err)
	}
	if err := merchantauth.VerifyEd25519(
		bytes.Repeat([]byte{0x55}, ed25519.PublicKeySize),
		base,
		parsedSignature,
	); err == nil {
		t.Fatal("webhook vector must reject the wrong Ed25519 public key")
	}

	got, err := c.VerifyWebhook(r)
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if string(got) != v.Body {
		t.Fatalf("webhook body mismatch")
	}
	payload, err := c.ParsePaymentWebhook(newWebhookVectorRequest(t, v))
	if err != nil {
		t.Fatalf("ParsePaymentWebhook: %v", err)
	}
	if payload.EventID != v.Headers[merchantauth.HeaderWebhookEventID] || payload.Status != StatusSucceeded {
		t.Fatalf("unexpected webhook payload: %+v", payload)
	}
}

func newWebhookVectorRequest(t *testing.T, v webhookVector) *http.Request {
	t.Helper()
	r, err := http.NewRequest(v.Input.Method, webhookVectorURL(v), bytes.NewReader([]byte(v.Body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for name, value := range v.Headers {
		r.Header.Set(name, value)
	}
	return r
}

func webhookVectorURL(v webhookVector) string {
	target := "https://merchant.example.com" + v.Input.Path
	if v.Input.RawQuery != "" {
		target += "?" + v.Input.RawQuery
	}
	return target
}

func TestWebhookVectorRejectsTamperedBody(t *testing.T) {
	v := readProtocolVector[webhookVector](t, "webhook/001-payment-succeeded.json")
	sig := readSignatureVector(t, "001-read-query.json")
	bodyKey := readProtocolVector[bodyCryptVector](t, "bodycrypt/001-sealed-box.json")
	c, err := NewClient(Config{
		BaseURL:                     "https://api.example.com",
		AccessKey:                   sig.Input.AccessKey,
		MerchantPrivateKeyBase64:    sig.Key.MerchantPrivateKeyBase64,
		PlatformBodyKeyID:           bodyKey.KeyID,
		PlatformBodyPublicKeyBase64: bodyKey.PlatformBodyPublicKeyBase64,
		PlatformWebhookPublicKeys: map[string]string{
			v.Key.PlatformWebhookKeyID: v.Key.PlatformWebhookPublicKeyB64,
		},
	}, WithClock(func() time.Time { return time.Unix(1787803200, 0) }))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	tampered := strings.Replace(v.Body, `"amount":"100.50"`, `"amount":"999.99"`, 1)
	r, err := http.NewRequest(v.Input.Method, webhookVectorURL(v), bytes.NewReader([]byte(tampered)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for name, value := range v.Headers {
		r.Header.Set(name, value)
	}
	if _, err := c.VerifyWebhook(r); err == nil {
		t.Fatal("want webhook vector body rejection")
	}
}
