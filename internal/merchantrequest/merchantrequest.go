package merchantrequest

import (
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/url"
	"time"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
)

var ErrInvalidRequest = errors.New("sdk: invalid signed request")

type Ed25519Signer struct {
	PrivateKey ed25519.PrivateKey
}

func (s Ed25519Signer) Sign(label string, base []byte) (string, error) {
	return merchantauth.SignEd25519(s.PrivateKey, label, base)
}

type BuildRequest struct {
	EndpointURL    string
	AccessKey      string
	IdempotencyKey string
	BodyKeyID      string
	BodyPublicKey  []byte
	Signer         Ed25519Signer
	Body           []byte
	Now            time.Time
	Nonce          string
}

type BuiltRequest struct {
	Method  string
	Body    []byte
	Headers map[string]string
}

func BuildWrite(req BuildRequest) (BuiltRequest, error) {
	if req.Now.IsZero() {
		req.Now = time.Now()
	}
	if req.EndpointURL == "" || req.AccessKey == "" || req.IdempotencyKey == "" ||
		req.BodyKeyID == "" || len(req.Body) == 0 {
		return BuiltRequest{}, ErrInvalidRequest
	}
	if err := merchantauth.ValidateIdempotencyKey(req.IdempotencyKey); err != nil {
		return BuiltRequest{}, err
	}
	u, err := url.Parse(req.EndpointURL)
	if err != nil || u.RawQuery != "" {
		return BuiltRequest{}, ErrInvalidRequest
	}
	wireBody, err := merchantauth.SealBodyEnvelope(req.Body, req.BodyPublicKey, req.BodyKeyID)
	if err != nil {
		return BuiltRequest{}, err
	}
	params := merchantauth.NewMerchantWriteSignatureParams(req.Nonce, req.Now)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set(merchantauth.HeaderContentEncryption, merchantauth.ContentEncryption)
	headers.Set(merchantauth.HeaderContentDigest, merchantauth.ContentDigestSHA256(wireBody))
	headers.Set(merchantauth.HeaderIdempotencyKey, req.IdempotencyKey)
	headers.Set(merchantauth.HeaderMerchantAccessKey, req.AccessKey)
	signatureInput, err := merchantauth.SignatureInputHeader(params)
	if err != nil {
		return BuiltRequest{}, err
	}
	headers.Set(merchantauth.HeaderSignatureInput, signatureInput)
	base, err := merchantauth.SignatureBase(&http.Request{Method: http.MethodPost, URL: u, Header: headers}, params)
	if err != nil {
		return BuiltRequest{}, err
	}
	signature, err := req.Signer.Sign(merchantauth.SignatureLabelMerchant, base)
	if err != nil {
		return BuiltRequest{}, err
	}
	headers.Set(merchantauth.HeaderSignature, signature)
	return BuiltRequest{Method: http.MethodPost, Body: wireBody, Headers: flattenHeaders(headers)}, nil
}

func BuildRead(req BuildRequest) (BuiltRequest, error) {
	if req.Now.IsZero() {
		req.Now = time.Now()
	}
	if req.EndpointURL == "" ||
		req.AccessKey == "" ||
		req.IdempotencyKey != "" ||
		req.BodyKeyID != "" ||
		len(req.BodyPublicKey) != 0 ||
		len(req.Body) != 0 {
		return BuiltRequest{}, ErrInvalidRequest
	}
	u, err := url.Parse(req.EndpointURL)
	if err != nil {
		return BuiltRequest{}, ErrInvalidRequest
	}
	params := merchantauth.NewMerchantReadSignatureParams(req.Nonce, req.Now)
	headers := http.Header{}
	headers.Set(merchantauth.HeaderMerchantAccessKey, req.AccessKey)
	signatureInput, err := merchantauth.SignatureInputHeader(params)
	if err != nil {
		return BuiltRequest{}, err
	}
	headers.Set(merchantauth.HeaderSignatureInput, signatureInput)
	base, err := merchantauth.SignatureBase(&http.Request{Method: http.MethodGet, URL: u, Header: headers}, params)
	if err != nil {
		return BuiltRequest{}, err
	}
	signature, err := req.Signer.Sign(merchantauth.SignatureLabelMerchant, base)
	if err != nil {
		return BuiltRequest{}, err
	}
	headers.Set(merchantauth.HeaderSignature, signature)
	return BuiltRequest{Method: http.MethodGet, Headers: flattenHeaders(headers)}, nil
}

func flattenHeaders(headers http.Header) map[string]string {
	out := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}
