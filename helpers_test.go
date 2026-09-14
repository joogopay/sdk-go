package joogopay

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
	"golang.org/x/crypto/nacl/box"
)

type testKeys struct {
	merchantPriv ed25519.PrivateKey
	merchantPub  ed25519.PublicKey
	bodyPub      *[32]byte
	bodyPriv     *[32]byte
	webhookPriv  ed25519.PrivateKey
	webhookPub   ed25519.PublicKey
	webhookKeyID string
}

func newTestKeys(t *testing.T) testKeys {
	t.Helper()
	mpub, mpriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bpub, bpriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wpub, wpriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return testKeys{
		merchantPriv: mpriv, merchantPub: mpub,
		bodyPub: bpub, bodyPriv: bpriv,
		webhookPriv: wpriv, webhookPub: wpub, webhookKeyID: "wk_1",
	}
}

func (k testKeys) config(baseURL string) Config {
	return Config{
		BaseURL:                     baseURL,
		AccessKey:                   "mak_live_test",
		MerchantPrivateKeyBase64:    base64.StdEncoding.EncodeToString(k.merchantPriv),
		PlatformBodyKeyID:           "body_1",
		PlatformBodyPublicKeyBase64: base64.StdEncoding.EncodeToString(k.bodyPub[:]),
		PlatformWebhookPublicKeys:   map[string]string{k.webhookKeyID: base64.StdEncoding.EncodeToString(k.webhookPub)},
	}
}

func (k testKeys) client(t *testing.T, baseURL string, httpClients ...*http.Client) *Client {
	t.Helper()
	cfg := k.config(baseURL)
	if len(httpClients) > 0 {
		cfg.HTTPClient = httpClients[0]
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

type capture struct {
	openedBody     []byte
	query          string
	idempotencyKey string
}

// verifyingServer authenticates every signed request exactly like the gateway
// would, then returns dataJSON wrapped in a success envelope.
func verifyingServer(t *testing.T, k testKeys, dataJSON string, cap *capture) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if err := merchantauth.VerifyMerchantReadRequestShape(r); err != nil {
				t.Errorf("read shape: %v", err)
			}
			params, err := merchantauth.ParseMerchantReadSignatureInput(r.Header.Get(merchantauth.HeaderSignatureInput))
			if err != nil {
				t.Errorf("read parse sig-input: %v", err)
			}
			if err := merchantauth.ValidateSignatureParams(params, time.Now()); err != nil {
				t.Errorf("read validate params: %v", err)
			}
			base, err := merchantauth.SignatureBase(r, params)
			if err != nil {
				t.Errorf("read base: %v", err)
			}
			sig, err := merchantauth.ParseSignature(r.Header.Get(merchantauth.HeaderSignature), merchantauth.SignatureLabelMerchant)
			if err != nil {
				t.Errorf("read parse sig: %v", err)
			}
			if err := merchantauth.VerifyEd25519(k.merchantPub, base, sig); err != nil {
				t.Errorf("read verify: %v", err)
			}
			if cap != nil {
				cap.query = r.URL.RawQuery
			}
		} else {
			if err := merchantauth.VerifyMerchantWriteRequestShape(r); err != nil {
				t.Errorf("write shape: %v", err)
			}
			body, err := merchantauth.ReadBody(r)
			if err != nil {
				t.Errorf("read body: %v", err)
			}
			if !merchantauth.VerifyContentDigest(body, r.Header.Get(merchantauth.HeaderContentDigest)) {
				t.Errorf("digest mismatch")
			}
			params, err := merchantauth.ParseMerchantWriteSignatureInput(r.Header.Get(merchantauth.HeaderSignatureInput))
			if err != nil {
				t.Errorf("write parse sig-input: %v", err)
			}
			if err := merchantauth.ValidateSignatureParams(params, time.Now()); err != nil {
				t.Errorf("write validate params: %v", err)
			}
			base, err := merchantauth.SignatureBase(r, params)
			if err != nil {
				t.Errorf("write base: %v", err)
			}
			sig, err := merchantauth.ParseSignature(r.Header.Get(merchantauth.HeaderSignature), merchantauth.SignatureLabelMerchant)
			if err != nil {
				t.Errorf("write parse sig: %v", err)
			}
			if err := merchantauth.VerifyEd25519(k.merchantPub, base, sig); err != nil {
				t.Errorf("write verify: %v", err)
			}
			plain, _, err := merchantauth.OpenBodyEnvelope(body, k.bodyPub[:], k.bodyPriv[:])
			if err != nil {
				t.Errorf("open envelope: %v", err)
			}
			if cap != nil {
				cap.openedBody = plain
				cap.idempotencyKey = r.Header.Get(merchantauth.HeaderIdempotencyKey)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"msg":"OK","traceId":"t1","data":` + dataJSON + `}`))
	}))
}

func errorServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":40400,"msg":"ORDER_NOT_FOUND","traceId":"t"}`))
	}))
}

func newEventID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	const crock = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	out := make([]byte, 26)
	for i := range out {
		out[i] = crock[int(b[i%16])%len(crock)]
	}
	return "evt_" + string(out)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
