package joogopay

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joogopay/sdk-go/internal/merchantauth"
)

// The signature vectors are shared by every SDK and pin the Signature-Input,
// the signature base and the Ed25519 signature byte for byte.

type signatureVector struct {
	Input struct {
		AccessKey string `json:"accessKey"`
		Method    string `json:"method"`
		Nonce     string `json:"nonce"`
		Path      string `json:"path"`
		RawQuery  string `json:"rawQuery"`
	} `json:"input"`
	Key struct {
		MerchantPrivateKeyBase64     string `json:"merchantPrivateKeyBase64"`
		MerchantPrivateKeySeedBase64 string `json:"merchantPrivateKeySeedBase64"`
		MerchantPublicKeyBase64      string `json:"merchantPublicKeyBase64"`
	} `json:"key"`
	Expected struct {
		SignatureInput string `json:"signatureInput"`
		SignatureBase  string `json:"signatureBase"`
		Signature      string `json:"signature"`
	} `json:"expected"`
}

func readSignatureVector(t *testing.T, name string) signatureVector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("protocol", "testdata", "signature", name))
	if err != nil {
		t.Fatalf("read vector %s: %v", name, err)
	}
	var v signatureVector
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse vector %s: %v", name, err)
	}
	return v
}

// requestFromSignatureBase rebuilds the request from the header values in
// expected.signatureBase so the test does not invent a second copy of the data.
func requestFromSignatureBase(t *testing.T, v signatureVector) *http.Request {
	t.Helper()
	target := v.Input.Path
	if v.Input.RawQuery != "" {
		target += "?" + v.Input.RawQuery
	}
	r := httptest.NewRequest(v.Input.Method, target, nil)
	names := map[string]string{
		"content-type":        "Content-Type",
		"content-encryption":  merchantauth.HeaderContentEncryption,
		"content-digest":      merchantauth.HeaderContentDigest,
		"idempotency-key":     merchantauth.HeaderIdempotencyKey,
		"merchant-access-key": merchantauth.HeaderMerchantAccessKey,
	}
	for _, line := range strings.Split(v.Expected.SignatureBase, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		var name string
		if err := json.Unmarshal([]byte(key), &name); err != nil {
			continue
		}
		if header, ok := names[name]; ok {
			r.Header.Set(header, value)
		}
	}
	return r
}

func TestSignatureVectors(t *testing.T) {
	cases := []struct {
		name    string
		covered []string
	}{
		{"001-read-query.json", merchantauth.MerchantReadCoveredComponents()},
		{"002-write-envelope.json", merchantauth.MerchantWriteCoveredComponents()},
		{"003-read-raw-query-unicode.json", merchantauth.MerchantReadCoveredComponents()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := readSignatureVector(t, tc.name)
			params := merchantauth.SignatureParams{
				Label:   merchantauth.SignatureLabelMerchant,
				Covered: tc.covered,
				Created: 1787803200,
				Expires: 1787803500,
				Nonce:   v.Input.Nonce,
				Alg:     merchantauth.SignatureAlgEd25519,
			}

			gotInput, err := merchantauth.SignatureInputHeader(params)
			if err != nil {
				t.Fatalf("signature input: %v", err)
			}
			if gotInput != v.Expected.SignatureInput {
				t.Fatalf("signature input mismatch:\n got %s\nwant %s", gotInput, v.Expected.SignatureInput)
			}
			// merchant signatures carry no keyid; accessKey already identifies the merchant
			if strings.Contains(gotInput, "keyid") {
				t.Fatalf("merchant signature input must not carry keyid: %s", gotInput)
			}

			base, err := merchantauth.SignatureBase(requestFromSignatureBase(t, v), params)
			if err != nil {
				t.Fatalf("signature base: %v", err)
			}
			if string(base) != v.Expected.SignatureBase {
				t.Fatalf("signature base mismatch:\n got %q\nwant %q", base, v.Expected.SignatureBase)
			}

			// Ed25519 is deterministic, so the signature matches byte for byte
			full := decodeVectorKey(t, v.Key.MerchantPrivateKeyBase64)
			got, err := merchantauth.SignEd25519(ed25519.PrivateKey(full), merchantauth.SignatureLabelMerchant, base)
			if err != nil {
				t.Fatalf("sign: %v", err)
			}
			if got != v.Expected.Signature {
				t.Fatalf("signature mismatch:\n got %s\nwant %s", got, v.Expected.Signature)
			}

			signature, err := merchantauth.ParseSignature(v.Expected.Signature, merchantauth.SignatureLabelMerchant)
			if err != nil {
				t.Fatalf("parse signature: %v", err)
			}
			if err := merchantauth.VerifyEd25519(decodeVectorKey(t, v.Key.MerchantPublicKeyBase64), base, signature); err != nil {
				t.Fatalf("verify: %v", err)
			}
		})
	}
}

// TestSignatureVectorSeedPrivateKey pins that a 32-byte seed and the 64-byte
// private key are interchangeable: libsodium and OpenSSL hand merchants the
// seed, and every SDK accepts it.
func TestSignatureVectorSeedPrivateKey(t *testing.T) {
	for _, name := range []string{"001-read-query.json", "002-write-envelope.json"} {
		t.Run(name, func(t *testing.T) {
			v := readSignatureVector(t, name)

			seed := decodeVectorKey(t, v.Key.MerchantPrivateKeySeedBase64)
			if len(seed) != ed25519.SeedSize {
				t.Fatalf("vector seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
			}
			full := decodeVectorKey(t, v.Key.MerchantPrivateKeyBase64)
			if len(full) != ed25519.PrivateKeySize {
				t.Fatalf("vector private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(full))
			}

			// the expanded seed is the vector's full private key
			expanded, err := decodeEd25519PrivateKey(v.Key.MerchantPrivateKeySeedBase64)
			if err != nil {
				t.Fatalf("decode seed private key: %v", err)
			}
			if !expanded.Equal(ed25519.PrivateKey(full)) {
				t.Fatal("seed-expanded private key differs from the 64-byte vector key")
			}

			// both forms produce the same signature
			params := merchantauth.SignatureParams{
				Label:   merchantauth.SignatureLabelMerchant,
				Covered: merchantauth.MerchantReadCoveredComponents(),
				Created: 1787803200,
				Expires: 1787803500,
				Nonce:   v.Input.Nonce,
				Alg:     merchantauth.SignatureAlgEd25519,
			}
			if name == "002-write-envelope.json" {
				params.Covered = merchantauth.MerchantWriteCoveredComponents()
			}
			base, err := merchantauth.SignatureBase(requestFromSignatureBase(t, v), params)
			if err != nil {
				t.Fatalf("signature base: %v", err)
			}
			fromSeed, err := merchantauth.SignEd25519(expanded, merchantauth.SignatureLabelMerchant, base)
			if err != nil {
				t.Fatalf("sign with seed: %v", err)
			}
			if fromSeed != v.Expected.Signature {
				t.Fatalf("seed signature mismatch:\n got %s\nwant %s", fromSeed, v.Expected.Signature)
			}
		})
	}
}

func TestClientAcceptsSeedAndFullPrivateKey(t *testing.T) {
	v := readSignatureVector(t, "001-read-query.json")
	for _, tc := range []struct {
		name string
		key  string
	}{
		{"32-byte seed", v.Key.MerchantPrivateKeySeedBase64},
		{"64-byte private key", v.Key.MerchantPrivateKeyBase64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(Config{
				BaseURL:                     "https://api.example.com",
				AccessKey:                   v.Input.AccessKey,
				MerchantPrivateKeyBase64:    tc.key,
				PlatformBodyKeyID:           "bodykey_1",
				PlatformBodyPublicKeyBase64: base64.StdEncoding.EncodeToString(make([]byte, merchantauth.X25519PublicKeySize)),
				PlatformWebhookPublicKeys:   map[string]string{"whk_1": v.Key.MerchantPublicKeyBase64},
			}); err != nil {
				t.Fatalf("NewClient rejected a valid %s: %v", tc.name, err)
			}
		})
	}

	for _, bad := range []string{
		base64.StdEncoding.EncodeToString(make([]byte, 31)),
		base64.StdEncoding.EncodeToString(make([]byte, 48)),
		base64.StdEncoding.EncodeToString(make([]byte, 65)),
	} {
		if _, err := decodeEd25519PrivateKey(bad); err != ErrInvalidMerchantPrivateKey {
			t.Fatalf("expected ErrInvalidMerchantPrivateKey for a wrong-length key, got %v", err)
		}
	}
}

func decodeVectorKey(t *testing.T, value string) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("decode key %q: %v", value, err)
	}
	return raw
}
