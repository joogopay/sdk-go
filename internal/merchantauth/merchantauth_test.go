package merchantauth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/nacl/box"
)

const validNonce = "b7754a6c-4a9c-4cf0-b77f-6f2d4b7e5f5a"

func TestContentDigest(t *testing.T) {
	body := []byte(`{"a":1}`)
	h := ContentDigestSHA256(body)
	if !VerifyContentDigest(body, h) {
		t.Fatal("digest should verify")
	}
	if VerifyContentDigest([]byte("other"), h) {
		t.Fatal("digest should not verify for other body")
	}
}

func TestSealOpenEnvelope(t *testing.T) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte(`{"amount":"1.00"}`)
	env, err := SealBodyEnvelope(plain, pub[:], "body_1")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if PeekBodyEnvelopeKeyID(env) != "body_1" {
		t.Fatal("peek keyid")
	}
	got, keyID, err := OpenBodyEnvelope(env, pub[:], priv[:])
	if err != nil || keyID != "body_1" || !bytes.Equal(got, plain) {
		t.Fatalf("open: %v %s", err, got)
	}

	if _, err := SealBodyEnvelope(nil, pub[:], "k"); err == nil {
		t.Fatal("empty plaintext")
	}
	if _, err := SealBodyEnvelope(plain, []byte("short"), "k"); err == nil {
		t.Fatal("bad pubkey size")
	}
	if _, err := SealBodyEnvelope(plain, pub[:], " "); err == nil {
		t.Fatal("blank keyid")
	}
	if _, _, err := OpenBodyEnvelope(env, []byte("short"), priv[:]); err == nil {
		t.Fatal("bad open pubkey size")
	}
	if _, _, err := OpenBodyEnvelope([]byte("not json"), pub[:], priv[:]); err == nil {
		t.Fatal("bad envelope json")
	}
	if _, _, err := OpenBodyEnvelope([]byte(`{"version":2,"alg":"x","keyId":"k","ciphertext":"AA"}`), pub[:], priv[:]); err == nil {
		t.Fatal("bad version")
	}
	wrongPub, wrongPriv, _ := box.GenerateKey(rand.Reader)
	if _, _, err := OpenBodyEnvelope(env, wrongPub[:], wrongPriv[:]); err == nil {
		t.Fatal("wrong key opens")
	}
	if PeekBodyEnvelopeKeyID([]byte("nope")) != "" {
		t.Fatal("peek bad")
	}
}

func TestSignVerifyAndHeaders(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	base := []byte("base-string")
	sigHeader, err := SignEd25519(priv, SignatureLabelMerchant, base)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sig, err := ParseSignature(sigHeader, SignatureLabelMerchant)
	if err != nil {
		t.Fatalf("parse sig: %v", err)
	}
	if err := VerifyEd25519(pub, base, sig); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if _, err := SignEd25519(priv, "", base); err == nil {
		t.Fatal("empty label")
	}
	if err := VerifyEd25519([]byte("short"), base, sig); err == nil {
		t.Fatal("bad pubkey")
	}
	if err := VerifyEd25519(pub, base, []byte("short")); err == nil {
		t.Fatal("bad sig size")
	}
	if err := VerifyEd25519(pub, []byte("other"), sig); err == nil {
		t.Fatal("wrong base")
	}
	if _, err := ParseSignature("merchant=xx", SignatureLabelMerchant); err == nil {
		t.Fatal("bad sig header")
	}
	if _, err := ParseSignature("merchant=:!!:", SignatureLabelMerchant); err == nil {
		t.Fatal("bad base64 sig")
	}
	if _, err := SignatureHeader("", sig); err == nil {
		t.Fatal("empty label header")
	}
}

func TestSignatureInputRoundTrip(t *testing.T) {
	now := time.Unix(1787803200, 0)
	for _, params := range []SignatureParams{
		NewMerchantWriteSignatureParams(validNonce, now),
		NewMerchantReadSignatureParams(validNonce, now),
		NewPlatformSignatureParams("pwhk_1", validNonce, now),
	} {
		header, err := SignatureInputHeader(params)
		if err != nil {
			t.Fatalf("input header: %v", err)
		}
		if params.Label == SignatureLabelMerchant && strings.Contains(header, "keyid=") {
			t.Fatalf("merchant signature input should not contain keyid: %s", header)
		}
		var parsed SignatureParams
		switch params.Label {
		case SignatureLabelPlatform:
			parsed, err = ParsePlatformSignatureInput(header)
		default:
			if len(params.Covered) == len(MerchantWriteCoveredComponents()) {
				parsed, err = ParseMerchantWriteSignatureInput(header)
			} else {
				parsed, err = ParseMerchantReadSignatureInput(header)
			}
		}
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if parsed.KeyID != params.KeyID || parsed.Nonce != validNonce || parsed.Created != now.Unix() {
			t.Fatalf("round trip mismatch: %+v", parsed)
		}
	}

	if _, err := ParseMerchantWriteSignatureInput("garbage"); err == nil {
		t.Fatal("garbage sig-input")
	}
}

func TestFreshness(t *testing.T) {
	now := time.Unix(1787803200, 0)
	ok := NewMerchantWriteSignatureParams(validNonce, now)
	if err := ValidateSignatureParams(ok, now); err != nil {
		t.Fatalf("fresh: %v", err)
	}
	// expired
	if err := ValidateFreshness(ok, now.Add(10*time.Minute)); err == nil {
		t.Fatal("want expired")
	}
	// window too long
	bad := ok
	bad.Expires = ok.Created + 3600
	if err := ValidateFreshness(bad, now); err == nil {
		t.Fatal("want window too long")
	}
	// bad params
	invalid := ok
	invalid.Alg = "hs256"
	if err := ValidateSignatureParams(invalid, now); err == nil {
		t.Fatal("want bad alg")
	}
	invalid = ok
	invalid.Nonce = "not-uuid"
	if err := ValidateSignatureParams(invalid, now); err == nil {
		t.Fatal("want bad nonce")
	}
}

func TestSmallValidators(t *testing.T) {
	if err := ValidateIdempotencyKey("018fb9b4-95f3-4a47-8f08-27466f7d4c1d"); err != nil {
		t.Fatal("valid idem")
	}
	if err := ValidateIdempotencyKey("nope"); err == nil {
		t.Fatal("bad idem")
	}
	if !ValidUUIDv4("018fb9b4-95f3-4a47-8f08-27466f7d4c1d") {
		t.Fatal("valid uuid")
	}
	for _, bad := range []string{"", "short", "018fb9b4x95f3-4a47-8f08-27466f7d4c1d", "018fb9b4-95f3-1a47-8f08-27466f7d4c1d", "018fb9b4-95f3-4a47-1f08-27466f7d4c1d", "018fb9b4-95f3-4a47-8f08-27466f7d4cGg"} {
		if ValidUUIDv4(bad) {
			t.Fatalf("uuid %q should be invalid", bad)
		}
	}
	if err := ValidateWebhookEventID("evt_00000000000000000000000000"); err != nil {
		t.Fatalf("valid event id: %v", err)
	}
	if err := ValidateWebhookEventID("bad"); err == nil {
		t.Fatal("bad event id")
	}
	if err := ValidateWebhookEventID("evt_0000000000000000000000000L"); err == nil {
		t.Fatal("event id bad char")
	}
}

func TestReadQueryAndPath(t *testing.T) {
	if err := ValidateMerchantReadQuery("orderNo=P1&currency=BRL"); err != nil {
		t.Fatalf("allowed: %v", err)
	}
	if err := ValidateMerchantReadQuery("payerCPF=123"); err == nil {
		t.Fatal("disallowed key")
	}
	if err := ValidateMerchantReadQuery("orderNo=P1&orderNo=P2"); err == nil {
		t.Fatal("multi value")
	}
	if err := ValidateMerchantReadQuery("orderNo="); err == nil {
		t.Fatal("empty value")
	}
	if err := ValidateMerchantReadQuery("%zz"); err == nil {
		t.Fatal("bad query")
	}

	req, _ := http.NewRequest(http.MethodGet, "https://x/api/v1/payments", nil)
	if EscapedPath(req.URL) != "/api/v1/payments" {
		t.Fatal("escaped path")
	}
	if EscapedPath(nil) != "/" {
		t.Fatal("nil path")
	}
}

// buildWriteRequest crafts a valid signed write request for shape/base tests.
func buildWriteRequest(t *testing.T) (*http.Request, []byte) {
	t.Helper()
	pub, priv, _ := box.GenerateKey(rand.Reader)
	_, mpriv, _ := ed25519.GenerateKey(rand.Reader)
	wire, err := SealBodyEnvelope([]byte(`{"a":1}`), pub[:], "body_1")
	if err != nil {
		t.Fatal(err)
	}
	_ = priv
	r, _ := http.NewRequest(http.MethodPost, "https://x/api/v1/payments", bytes.NewReader(wire))
	r.Body = io.NopCloser(bytes.NewReader(wire))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(HeaderContentEncryption, ContentEncryption)
	r.Header.Set(HeaderContentDigest, ContentDigestSHA256(wire))
	r.Header.Set(HeaderIdempotencyKey, "018fb9b4-95f3-4a47-8f08-27466f7d4c1d")
	r.Header.Set(HeaderMerchantAccessKey, "mak_live")
	params := NewMerchantWriteSignatureParams(validNonce, time.Now())
	si, _ := SignatureInputHeader(params)
	r.Header.Set(HeaderSignatureInput, si)
	base, _ := SignatureBase(r, params)
	sig, _ := SignEd25519(mpriv, SignatureLabelMerchant, base)
	r.Header.Set(HeaderSignature, sig)
	return r, wire
}

func TestWriteShapeAndBase(t *testing.T) {
	r, _ := buildWriteRequest(t)
	if err := VerifyMerchantWriteRequestShape(r); err != nil {
		t.Fatalf("valid write shape: %v", err)
	}
	r2, _ := buildWriteRequest(t)
	r2.Header.Del(HeaderContentDigest)
	if err := VerifyMerchantWriteRequestShape(r2); err == nil {
		t.Fatal("missing digest")
	}
	r3, _ := buildWriteRequest(t)
	r3.Header.Set(HeaderIdempotencyKey, "bad")
	if err := VerifyMerchantWriteRequestShape(r3); err == nil {
		t.Fatal("bad idempotency")
	}
	r4, _ := buildWriteRequest(t)
	r4.Method = http.MethodGet
	if err := VerifyMerchantWriteRequestShape(r4); err == nil {
		t.Fatal("wrong method")
	}
}

func TestReadShape(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "https://x/api/v1/payments?orderNo=P1", nil)
	r.Header.Set(HeaderMerchantAccessKey, "mak_live")
	params := NewMerchantReadSignatureParams(validNonce, time.Now())
	si, _ := SignatureInputHeader(params)
	r.Header.Set(HeaderSignatureInput, si)
	base, _ := SignatureBase(r, params)
	_, mpriv, _ := ed25519.GenerateKey(rand.Reader)
	sig, _ := SignEd25519(mpriv, SignatureLabelMerchant, base)
	r.Header.Set(HeaderSignature, sig)
	if err := VerifyMerchantReadRequestShape(r); err != nil {
		t.Fatalf("valid read shape: %v", err)
	}
	// GET must not carry Content-Type
	r.Header.Set("Content-Type", "application/json")
	if err := VerifyMerchantReadRequestShape(r); err == nil {
		t.Fatal("read with content-type")
	}
}

func TestReadBodyLimit(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "https://x/y", bytes.NewReader([]byte("hello")))
	r.Body = io.NopCloser(bytes.NewReader([]byte("hello")))
	body, err := ReadBody(r)
	if err != nil || string(body) != "hello" {
		t.Fatalf("read body: %v %s", err, body)
	}
	empty, _ := http.NewRequest(http.MethodPost, "https://x/y", nil)
	if _, err := ReadBody(empty); err == nil {
		t.Fatal("nil body")
	}
}
