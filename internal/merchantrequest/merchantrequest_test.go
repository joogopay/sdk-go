package merchantrequest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/http"
	"testing"
	"time"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
	"golang.org/x/crypto/nacl/box"
)

func keys(t *testing.T) (ed25519.PrivateKey, ed25519.PublicKey, *[32]byte, *[32]byte) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bpub, bpriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv, pub, bpub, bpriv
}

const validIdem = "018fb9b4-95f3-4a47-8f08-27466f7d4c1d"

func writeReq(priv ed25519.PrivateKey, bodyPub *[32]byte) BuildRequest {
	return BuildRequest{
		EndpointURL:    "https://api.example.com/api/v1/payments",
		AccessKey:      "mak_live",
		IdempotencyKey: validIdem,
		BodyKeyID:      "body_1",
		BodyPublicKey:  bodyPub[:],
		Signer:         Ed25519Signer{PrivateKey: priv},
		Body:           []byte(`{"amount":"1.00"}`),
		Now:            time.Now(),
		Nonce:          "b7754a6c-4a9c-4cf0-b77f-6f2d4b7e5f5a",
	}
}

func TestBuildWriteRoundTrip(t *testing.T) {
	priv, pub, bpub, bpriv := keys(t)
	built, err := BuildWrite(writeReq(priv, bpub))
	if err != nil {
		t.Fatalf("BuildWrite: %v", err)
	}
	if built.Method != http.MethodPost || len(built.Body) == 0 {
		t.Fatalf("bad built: %+v", built)
	}

	r, _ := http.NewRequest(http.MethodPost, "https://api.example.com/api/v1/payments", bytes.NewReader(built.Body))
	r.Body = io.NopCloser(bytes.NewReader(built.Body))
	for k, v := range built.Headers {
		r.Header.Set(k, v)
	}

	if err := merchantauth.VerifyMerchantWriteRequestShape(r); err != nil {
		t.Fatalf("shape: %v", err)
	}
	if !merchantauth.VerifyContentDigest(built.Body, r.Header.Get(merchantauth.HeaderContentDigest)) {
		t.Fatal("digest")
	}
	params, err := merchantauth.ParseMerchantWriteSignatureInput(r.Header.Get(merchantauth.HeaderSignatureInput))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	base, err := merchantauth.SignatureBase(r, params)
	if err != nil {
		t.Fatalf("base: %v", err)
	}
	sig, err := merchantauth.ParseSignature(r.Header.Get(merchantauth.HeaderSignature), merchantauth.SignatureLabelMerchant)
	if err != nil {
		t.Fatalf("parse sig: %v", err)
	}
	if err := merchantauth.VerifyEd25519(pub, base, sig); err != nil {
		t.Fatalf("verify: %v", err)
	}
	plain, _, err := merchantauth.OpenBodyEnvelope(built.Body, bpub[:], bpriv[:])
	if err != nil || string(plain) != `{"amount":"1.00"}` {
		t.Fatalf("open: %v %s", err, plain)
	}
}

func TestBuildWriteValidation(t *testing.T) {
	priv, _, bpub, _ := keys(t)
	base := writeReq(priv, bpub)

	if _, err := BuildWrite(BuildRequest{}); err != ErrInvalidRequest {
		t.Fatal("empty")
	}
	bad := base
	bad.IdempotencyKey = "not-a-uuid"
	if _, err := BuildWrite(bad); err == nil {
		t.Fatal("bad idempotency")
	}
	bad = base
	bad.EndpointURL = "https://api.example.com/api/v1/payments?x=1"
	if _, err := BuildWrite(bad); err != ErrInvalidRequest {
		t.Fatal("query in write endpoint")
	}
	bad = base
	bad.BodyPublicKey = []byte("short")
	if _, err := BuildWrite(bad); err == nil {
		t.Fatal("bad body key")
	}

	// zero Now defaults to time.Now
	ok := base
	ok.Now = time.Time{}
	if _, err := BuildWrite(ok); err != nil {
		t.Fatalf("zero now: %v", err)
	}
}

func TestBuildReadRoundTrip(t *testing.T) {
	priv, pub, _, _ := keys(t)
	built, err := BuildRead(BuildRequest{
		EndpointURL: "https://api.example.com/api/v1/payments?orderNo=P1",
		AccessKey:   "mak_live",
		Signer:      Ed25519Signer{PrivateKey: priv},
		Now:         time.Now(),
		Nonce:       "b7754a6c-4a9c-4cf0-b77f-6f2d4b7e5f5a",
	})
	if err != nil {
		t.Fatalf("BuildRead: %v", err)
	}
	if built.Method != http.MethodGet || built.Body != nil {
		t.Fatalf("bad built: %+v", built)
	}
	r, _ := http.NewRequest(http.MethodGet, "https://api.example.com/api/v1/payments?orderNo=P1", nil)
	for k, v := range built.Headers {
		r.Header.Set(k, v)
	}
	if err := merchantauth.VerifyMerchantReadRequestShape(r); err != nil {
		t.Fatalf("shape: %v", err)
	}
	params, _ := merchantauth.ParseMerchantReadSignatureInput(r.Header.Get(merchantauth.HeaderSignatureInput))
	base, _ := merchantauth.SignatureBase(r, params)
	sig, _ := merchantauth.ParseSignature(r.Header.Get(merchantauth.HeaderSignature), merchantauth.SignatureLabelMerchant)
	if err := merchantauth.VerifyEd25519(pub, base, sig); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestBuildReadValidation(t *testing.T) {
	priv, _, _, _ := keys(t)
	if _, err := BuildRead(BuildRequest{}); err != ErrInvalidRequest {
		t.Fatal("empty")
	}
	if _, err := BuildRead(BuildRequest{EndpointURL: "https://x/y", AccessKey: "a", Body: []byte("b"), Signer: Ed25519Signer{PrivateKey: priv}}); err != ErrInvalidRequest {
		t.Fatal("read with body")
	}
	if _, err := BuildRead(BuildRequest{EndpointURL: "https://x/y", AccessKey: "a", IdempotencyKey: validIdem, Signer: Ed25519Signer{PrivateKey: priv}}); err != ErrInvalidRequest {
		t.Fatal("read with idempotency key")
	}
	if _, err := BuildRead(BuildRequest{EndpointURL: "https://x/y", AccessKey: "a", BodyKeyID: "body_1", Signer: Ed25519Signer{PrivateKey: priv}}); err != ErrInvalidRequest {
		t.Fatal("read with body key id")
	}
	if _, err := BuildRead(BuildRequest{EndpointURL: "://bad", AccessKey: "a", Signer: Ed25519Signer{PrivateKey: priv}}); err != ErrInvalidRequest {
		t.Fatal("bad url")
	}
}
