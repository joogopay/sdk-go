// This file is shared byte for byte (package name aside) with the public
// merchant SDK; the shared conformance vectors keep both sides in step.

// Package merchantauth implements the merchant API request signing, body sealing and
// webhook verification primitives shared by the platform and the SDKs.
package merchantauth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/nacl/box"
)

const (
	HeaderMerchantAccessKey = "Merchant-Access-Key"
	HeaderWebhookEventID    = "Webhook-Event-Id"
	HeaderContentEncryption = "Content-Encryption"
	HeaderContentDigest     = "Content-Digest"
	HeaderIdempotencyKey    = "Idempotency-Key"
	HeaderSignatureInput    = "Signature-Input"
	HeaderSignature         = "Signature"

	SignatureLabelMerchant = "merchant"
	SignatureLabelPlatform = "platform"
	SignatureAlgEd25519    = "ed25519"
	ContentEncryption      = "sealedbox-v1-x25519-xsalsa20poly1305"
	MaxPlainBodyBytes      = 1 << 20
	MaxWireBodyBytes       = 2 << 20
	MaxSignatureLifetime   = 5 * time.Minute
	X25519PublicKeySize    = 32
	X25519PrivateKeySize   = 32
)

var (
	ErrInvalidHeader    = errors.New("invalid signed header")
	ErrInvalidEnvelope  = errors.New("invalid body envelope")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrExpiredSignature = errors.New("expired signature")
)

var (
	merchantWriteCoveredComponents = []string{
		"@method",
		"@path",
		"content-type",
		"content-encryption",
		"content-digest",
		"idempotency-key",
		"merchant-access-key",
	}
	merchantReadCoveredComponents = []string{
		"@method",
		"@path",
		"@query",
		"merchant-access-key",
	}
	platformCoveredComponents = []string{
		"@method",
		"@path",
		"@query",
		"content-type",
		"content-digest",
		"webhook-event-id",
	}
)

// BodyEnvelope is the sealed-box wire body of a merchant POST.
type BodyEnvelope struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"alg"`
	KeyID      string `json:"keyId"`
	Ciphertext string `json:"ciphertext"`
}

// SignatureParams holds the signature parameters of the fixed RFC 9421 profile.
type SignatureParams struct {
	Label   string
	Covered []string
	Created int64
	Expires int64
	Nonce   string
	KeyID   string
	Alg     string
}

// MerchantWriteCoveredComponents returns a copy of the covered components of a merchant write signature.
func MerchantWriteCoveredComponents() []string {
	return append([]string(nil), merchantWriteCoveredComponents...)
}

// MerchantReadCoveredComponents returns a copy of the covered components of a merchant read signature.
func MerchantReadCoveredComponents() []string {
	return append([]string(nil), merchantReadCoveredComponents...)
}

// PlatformCoveredComponents returns a copy of the covered components of a platform webhook signature.
func PlatformCoveredComponents() []string {
	return append([]string(nil), platformCoveredComponents...)
}

// VerifyMerchantWriteRequestShape rejects a merchant POST whose method, query,
// content headers, signed headers or idempotency key deviate from the write profile.
func VerifyMerchantWriteRequestShape(r *http.Request) error {
	if r == nil || r.Method != http.MethodPost || r.URL == nil || r.URL.RawQuery != "" {
		return ErrInvalidHeader
	}
	if !singleHeaderEquals(r, "Content-Type", "application/json") || !contentEncodingAllowed(r) {
		return ErrInvalidHeader
	}
	for _, name := range []string{
		HeaderMerchantAccessKey,
		HeaderContentEncryption,
		HeaderContentDigest,
		HeaderSignatureInput,
		HeaderSignature,
		HeaderIdempotencyKey,
	} {
		if _, err := requiredHeader(r, name); err != nil {
			return ErrInvalidHeader
		}
	}
	if r.Header.Get(HeaderContentEncryption) != ContentEncryption {
		return ErrInvalidHeader
	}
	if err := ValidateIdempotencyKey(r.Header.Get(HeaderIdempotencyKey)); err != nil {
		return ErrInvalidHeader
	}
	return nil
}

// VerifyMerchantReadRequestShape rejects a merchant GET that carries a body,
// write-only headers or query fields outside the public locator set.
func VerifyMerchantReadRequestShape(r *http.Request) error {
	if r == nil || r.Method != http.MethodGet || r.URL == nil {
		return ErrInvalidHeader
	}
	if !contentEncodingAllowed(r) || r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
		return ErrInvalidHeader
	}
	if err := rejectReadableBody(r); err != nil {
		return ErrInvalidHeader
	}
	for _, name := range []string{
		"Content-Type",
		HeaderContentEncryption,
		HeaderContentDigest,
		HeaderIdempotencyKey,
	} {
		if len(r.Header.Values(name)) > 0 {
			return ErrInvalidHeader
		}
	}
	for _, name := range []string{
		HeaderMerchantAccessKey,
		HeaderSignatureInput,
		HeaderSignature,
	} {
		if _, err := requiredHeader(r, name); err != nil {
			return ErrInvalidHeader
		}
	}
	if err := ValidateMerchantReadQuery(r.URL.RawQuery); err != nil {
		return ErrInvalidHeader
	}
	return nil
}

// VerifyWebhookRequestShape rejects a platform webhook POST that lacks the
// signed headers or carries a malformed event id.
func VerifyWebhookRequestShape(r *http.Request) error {
	if r == nil || r.Method != http.MethodPost || r.URL == nil {
		return ErrInvalidHeader
	}
	if !singleHeaderEquals(r, "Content-Type", "application/json") || !contentEncodingAllowed(r) {
		return ErrInvalidHeader
	}
	for _, name := range []string{
		HeaderWebhookEventID,
		HeaderContentDigest,
		HeaderSignatureInput,
		HeaderSignature,
	} {
		if _, err := requiredHeader(r, name); err != nil {
			return ErrInvalidHeader
		}
	}
	return ValidateWebhookEventID(r.Header.Get(HeaderWebhookEventID))
}

// ReadBody reads the whole body and rewinds r.Body; an empty body or one over
// MaxWireBodyBytes returns ErrInvalidHeader.
func ReadBody(r *http.Request) ([]byte, error) {
	if r == nil || r.Body == nil || r.Body == http.NoBody {
		return nil, ErrInvalidHeader
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxWireBodyBytes+1))
	if err != nil || len(body) == 0 || len(body) > MaxWireBodyBytes {
		return nil, ErrInvalidHeader
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// ContentDigestSHA256 returns the RFC 9530 sha-256 Content-Digest value of body.
func ContentDigestSHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// VerifyContentDigest reports whether header is the sha-256 Content-Digest of body.
func VerifyContentDigest(body []byte, header string) bool {
	return header == ContentDigestSHA256(body)
}

// ValidateIdempotencyKey requires the write idempotency key to be a lowercase UUID v4.
func ValidateIdempotencyKey(value string) error {
	if !ValidUUIDv4(value) {
		return ErrInvalidHeader
	}
	return nil
}

// NewMerchantWriteSignatureParams builds merchant write signature parameters
// valid for MaxSignatureLifetime from now.
func NewMerchantWriteSignatureParams(nonce string, now time.Time) SignatureParams {
	return SignatureParams{
		Label:   SignatureLabelMerchant,
		Covered: MerchantWriteCoveredComponents(),
		Created: now.Unix(),
		Expires: now.Add(MaxSignatureLifetime).Unix(),
		Nonce:   nonce,
		Alg:     SignatureAlgEd25519,
	}
}

// NewMerchantReadSignatureParams builds merchant read signature parameters
// valid for MaxSignatureLifetime from now.
func NewMerchantReadSignatureParams(nonce string, now time.Time) SignatureParams {
	return SignatureParams{
		Label:   SignatureLabelMerchant,
		Covered: MerchantReadCoveredComponents(),
		Created: now.Unix(),
		Expires: now.Add(MaxSignatureLifetime).Unix(),
		Nonce:   nonce,
		Alg:     SignatureAlgEd25519,
	}
}

// NewPlatformSignatureParams builds platform webhook signature parameters
// valid for MaxSignatureLifetime from now.
func NewPlatformSignatureParams(keyID string, nonce string, now time.Time) SignatureParams {
	return SignatureParams{
		Label:   SignatureLabelPlatform,
		Covered: PlatformCoveredComponents(),
		Created: now.Unix(),
		Expires: now.Add(MaxSignatureLifetime).Unix(),
		Nonce:   nonce,
		KeyID:   keyID,
		Alg:     SignatureAlgEd25519,
	}
}

// SignatureInputHeader renders params as a Signature-Input header value.
func SignatureInputHeader(params SignatureParams) (string, error) {
	if err := ValidateSignatureParams(params, time.Unix(params.Created, 0)); err != nil {
		return "", err
	}
	return params.Label + "=" + signatureInputValue(params), nil
}

// ParseMerchantWriteSignatureInput parses a Signature-Input header against the merchant write profile.
func ParseMerchantWriteSignatureInput(value string) (SignatureParams, error) {
	return parseSignatureInput(value, SignatureLabelMerchant, merchantWriteCoveredComponents)
}

// ParseMerchantReadSignatureInput parses a Signature-Input header against the merchant read profile.
func ParseMerchantReadSignatureInput(value string) (SignatureParams, error) {
	return parseSignatureInput(value, SignatureLabelMerchant, merchantReadCoveredComponents)
}

// ParsePlatformSignatureInput parses a Signature-Input header against the platform webhook profile.
func ParsePlatformSignatureInput(value string) (SignatureParams, error) {
	return parseSignatureInput(value, SignatureLabelPlatform, platformCoveredComponents)
}

// ValidateSignatureParams checks label, algorithm, nonce, key id, the fixed
// covered component set and freshness against now.
func ValidateSignatureParams(params SignatureParams, now time.Time) error {
	if params.Label == "" || params.Alg != SignatureAlgEd25519 || !ValidUUIDv4(params.Nonce) {
		return ErrInvalidHeader
	}
	if params.Label == SignatureLabelPlatform {
		if !validHeaderValue(params.KeyID) {
			return ErrInvalidHeader
		}
	} else if params.Label != SignatureLabelMerchant {
		return ErrInvalidHeader
	}
	if !coveredComponentsAllowed(params.Label, params.Covered) {
		return ErrInvalidHeader
	}
	return ValidateFreshness(params, now)
}

// ValidateFreshness returns ErrExpiredSignature unless created < expires, the
// window is at most MaxSignatureLifetime, and now falls inside it, allowing
// MaxSignatureLifetime of clock skew before created.
func ValidateFreshness(params SignatureParams, now time.Time) error {
	if params.Created <= 0 || params.Expires <= 0 || params.Expires <= params.Created || params.Expires-params.Created > int64(MaxSignatureLifetime.Seconds()) {
		return ErrExpiredSignature
	}
	unixNow := now.Unix()
	if unixNow < params.Created-int64(MaxSignatureLifetime.Seconds()) || unixNow > params.Expires {
		return ErrExpiredSignature
	}
	return nil
}

// SignatureBase builds the RFC 9421 signature base of r over params.Covered.
func SignatureBase(r *http.Request, params SignatureParams) ([]byte, error) {
	if r == nil || r.URL == nil || len(params.Covered) == 0 {
		return nil, ErrInvalidHeader
	}
	lines := make([]string, 0, len(params.Covered)+1)
	for _, component := range params.Covered {
		value, err := componentValue(r, component)
		if err != nil {
			return nil, err
		}
		lines = append(lines, strconv.Quote(component)+": "+value)
	}
	lines = append(lines, strconv.Quote("@signature-params")+": "+signatureInputValue(params))
	return []byte(strings.Join(lines, "\n")), nil
}

// SignEd25519 signs base with privateKey and returns the Signature header value under label.
func SignEd25519(privateKey ed25519.PrivateKey, label string, base []byte) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize || label == "" || len(base) == 0 {
		return "", ErrInvalidSignature
	}
	return SignatureHeader(label, ed25519.Sign(privateKey, base))
}

// VerifyEd25519 returns ErrInvalidSignature unless signature is a valid Ed25519
// signature of base under publicKey.
func VerifyEd25519(publicKey []byte, base []byte, signature []byte) error {
	if len(publicKey) != ed25519.PublicKeySize || len(base) == 0 || len(signature) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), base, signature) {
		return ErrInvalidSignature
	}
	return nil
}

// ParseSignature extracts the raw Ed25519 signature from a Signature header value under label.
func ParseSignature(value string, label string) ([]byte, error) {
	if !validHeaderValue(label) {
		return nil, ErrInvalidSignature
	}
	prefix := label + "=:"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, ":") {
		return nil, ErrInvalidSignature
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(value, prefix), ":")
	signature, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return nil, ErrInvalidSignature
	}
	return signature, nil
}

// SignatureHeader renders signature as a Signature header value under label.
func SignatureHeader(label string, signature []byte) (string, error) {
	if !validHeaderValue(label) || len(signature) != ed25519.SignatureSize {
		return "", ErrInvalidSignature
	}
	return label + "=:" + base64.StdEncoding.EncodeToString(signature) + ":", nil
}

// SealBodyEnvelope seals plaintext to the platform X25519 public key and
// returns the JSON envelope; plaintext is limited to MaxPlainBodyBytes.
func SealBodyEnvelope(plaintext []byte, publicKey []byte, keyID string) ([]byte, error) {
	if len(plaintext) == 0 || len(plaintext) > MaxPlainBodyBytes || len(publicKey) != X25519PublicKeySize || !validHeaderValue(keyID) {
		return nil, ErrInvalidEnvelope
	}
	var pub [32]byte
	copy(pub[:], publicKey)
	ciphertext, err := box.SealAnonymous(nil, plaintext, &pub, rand.Reader)
	if err != nil {
		return nil, err
	}
	return json.Marshal(BodyEnvelope{
		Version:    1,
		Algorithm:  ContentEncryption,
		KeyID:      keyID,
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	})
}

// PeekBodyEnvelopeKeyID returns the key id of a well-formed envelope without
// decrypting it, or "" when the envelope is malformed.
func PeekBodyEnvelopeKeyID(envelopeBody []byte) string {
	envelope, err := decodeBodyEnvelope(envelopeBody)
	if err != nil {
		return ""
	}
	return envelope.KeyID
}

// OpenBodyEnvelope decrypts a merchant POST envelope with the platform X25519
// key pair and returns the plaintext and the envelope key id.
func OpenBodyEnvelope(envelopeBody []byte, publicKey []byte, privateKey []byte) ([]byte, string, error) {
	if len(publicKey) != X25519PublicKeySize || len(privateKey) != X25519PrivateKeySize {
		return nil, "", ErrInvalidEnvelope
	}
	envelope, err := decodeBodyEnvelope(envelopeBody)
	if err != nil {
		return nil, "", err
	}
	ciphertext, _ := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	var pub, priv [32]byte
	copy(pub[:], publicKey)
	copy(priv[:], privateKey)
	plaintext, ok := box.OpenAnonymous(nil, ciphertext, &pub, &priv)
	if !ok || len(plaintext) == 0 || len(plaintext) > MaxPlainBodyBytes {
		return nil, "", ErrInvalidEnvelope
	}
	return plaintext, envelope.KeyID, nil
}

// ValidateWebhookEventID requires "evt_" followed by 26 Crockford base32 characters.
func ValidateWebhookEventID(value string) error {
	if !strings.HasPrefix(value, "evt_") || len(value) != 30 {
		return ErrInvalidHeader
	}
	for _, r := range value[4:] {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) {
			return ErrInvalidHeader
		}
	}
	return nil
}

// EscapedPath returns the @path component of u; a nil URL or empty path signs as "/".
func EscapedPath(u *url.URL) string {
	if u == nil {
		return "/"
	}
	path := u.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

// ValidateMerchantReadQuery allows only the public locator fields, each given
// once with a non-blank value.
func ValidateMerchantReadQuery(rawQuery string) error {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return ErrInvalidHeader
	}
	for key, value := range values {
		if _, ok := allowedReadQueryFields[key]; !ok || len(value) != 1 || strings.TrimSpace(value[0]) == "" {
			return ErrInvalidHeader
		}
	}
	return nil
}

func componentValue(r *http.Request, component string) (string, error) {
	switch component {
	case "@method":
		return r.Method, nil
	case "@path":
		return EscapedPath(r.URL), nil
	case "@query":
		if r.URL == nil {
			return "", ErrInvalidHeader
		}
		return "?" + r.URL.RawQuery, nil
	case "content-type":
		return requiredHeader(r, "Content-Type")
	case "content-encryption":
		return requiredHeader(r, HeaderContentEncryption)
	case "content-digest":
		return requiredHeader(r, HeaderContentDigest)
	case "idempotency-key":
		return requiredHeader(r, HeaderIdempotencyKey)
	case "merchant-access-key":
		return requiredHeader(r, HeaderMerchantAccessKey)
	case "webhook-event-id":
		return requiredHeader(r, HeaderWebhookEventID)
	default:
		return "", ErrInvalidHeader
	}
}

func requiredHeader(r *http.Request, name string) (string, error) {
	if !singleValue(r, name) {
		return "", ErrInvalidHeader
	}
	value := strings.TrimSpace(r.Header.Get(name))
	if !validHeaderValue(value) {
		return "", ErrInvalidHeader
	}
	return value, nil
}

func singleHeaderEquals(r *http.Request, name string, expected string) bool {
	return singleValue(r, name) && strings.TrimSpace(r.Header.Get(name)) == expected
}

func singleValue(r *http.Request, name string) bool {
	values := r.Header.Values(name)
	return len(values) == 1
}

func contentEncodingAllowed(r *http.Request) bool {
	values := r.Header.Values("Content-Encoding")
	if len(values) == 0 {
		return true
	}
	return len(values) == 1 && strings.TrimSpace(values[0]) == "identity"
}

func rejectReadableBody(r *http.Request) error {
	if r.Body == nil || r.Body == http.NoBody {
		r.Body = http.NoBody
		return nil
	}
	probe, err := io.ReadAll(io.LimitReader(r.Body, 1))
	if err != nil || len(probe) > 0 {
		return ErrInvalidHeader
	}
	r.Body = http.NoBody
	return nil
}

func signatureInputValue(params SignatureParams) string {
	quoted := make([]string, 0, len(params.Covered))
	for _, component := range params.Covered {
		quoted = append(quoted, strconv.Quote(component))
	}
	value := "(" + strings.Join(quoted, " ") + ")" +
		";created=" + strconv.FormatInt(params.Created, 10) +
		";expires=" + strconv.FormatInt(params.Expires, 10) +
		";nonce=" + strconv.Quote(params.Nonce)
	if params.Label == SignatureLabelPlatform {
		value += ";keyid=" + strconv.Quote(params.KeyID)
	}
	return value + ";alg=" + strconv.Quote(params.Alg)
}

func parseSignatureInput(value string, label string, covered []string) (SignatureParams, error) {
	prefix := label + "=("
	if !strings.HasPrefix(value, prefix) {
		return SignatureParams{}, ErrInvalidHeader
	}
	closing := strings.Index(value, ")")
	if closing <= len(prefix)-1 || closing+1 >= len(value) || value[closing+1] != ';' {
		return SignatureParams{}, ErrInvalidHeader
	}
	rawComponents := strings.Fields(value[len(prefix):closing])
	if len(rawComponents) != len(covered) {
		return SignatureParams{}, ErrInvalidHeader
	}
	params := SignatureParams{Label: label, Covered: make([]string, 0, len(covered))}
	for i, raw := range rawComponents {
		component, err := strconv.Unquote(raw)
		if err != nil || component != covered[i] {
			return SignatureParams{}, ErrInvalidHeader
		}
		params.Covered = append(params.Covered, component)
	}
	parts := strings.Split(value[closing+2:], ";")
	expectedParts := 4
	if label == SignatureLabelPlatform {
		expectedParts = 5
	}
	if len(parts) != expectedParts {
		return SignatureParams{}, ErrInvalidHeader
	}
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		key, raw, ok := strings.Cut(part, "=")
		if !ok || key == "" {
			return SignatureParams{}, ErrInvalidHeader
		}
		if _, exists := seen[key]; exists {
			return SignatureParams{}, ErrInvalidHeader
		}
		seen[key] = struct{}{}
		switch key {
		case "created":
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return SignatureParams{}, ErrInvalidHeader
			}
			params.Created = v
		case "expires":
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return SignatureParams{}, ErrInvalidHeader
			}
			params.Expires = v
		case "nonce":
			v, err := strconv.Unquote(raw)
			if err != nil {
				return SignatureParams{}, ErrInvalidHeader
			}
			params.Nonce = v
		case "keyid":
			if label != SignatureLabelPlatform {
				return SignatureParams{}, ErrInvalidHeader
			}
			v, err := strconv.Unquote(raw)
			if err != nil {
				return SignatureParams{}, ErrInvalidHeader
			}
			params.KeyID = v
		case "alg":
			v, err := strconv.Unquote(raw)
			if err != nil {
				return SignatureParams{}, ErrInvalidHeader
			}
			params.Alg = v
		default:
			return SignatureParams{}, ErrInvalidHeader
		}
	}
	return params, nil
}

func decodeBodyEnvelope(envelopeBody []byte) (BodyEnvelope, error) {
	if len(envelopeBody) == 0 || len(envelopeBody) > MaxWireBodyBytes {
		return BodyEnvelope{}, ErrInvalidEnvelope
	}
	var envelope BodyEnvelope
	dec := json.NewDecoder(bytes.NewReader(envelopeBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&envelope); err != nil {
		return BodyEnvelope{}, ErrInvalidEnvelope
	}
	var extra struct{}
	if err := dec.Decode(&extra); err != io.EOF {
		return BodyEnvelope{}, ErrInvalidEnvelope
	}
	if envelope.Version != 1 ||
		envelope.Algorithm != ContentEncryption ||
		!validHeaderValue(envelope.KeyID) ||
		envelope.Ciphertext == "" {
		return BodyEnvelope{}, ErrInvalidEnvelope
	}
	if _, err := base64.StdEncoding.DecodeString(envelope.Ciphertext); err != nil {
		return BodyEnvelope{}, ErrInvalidEnvelope
	}
	return envelope, nil
}

func coveredComponentsAllowed(label string, covered []string) bool {
	switch label {
	case SignatureLabelMerchant:
		return stringSliceEqual(covered, merchantWriteCoveredComponents) || stringSliceEqual(covered, merchantReadCoveredComponents)
	case SignatureLabelPlatform:
		return stringSliceEqual(covered, platformCoveredComponents)
	default:
		return false
	}
}

func stringSliceEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func validHeaderValue(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\r\n")
}

// ValidUUIDv4 reports whether value is a lowercase, hyphenated UUID v4.
func ValidUUIDv4(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		case 14:
			if r != '4' {
				return false
			}
		case 19:
			if r != '8' && r != '9' && r != 'a' && r != 'b' {
				return false
			}
		default:
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				return false
			}
		}
	}
	return true
}

var allowedReadQueryFields = map[string]struct{}{
	"orderNo":                {},
	"merchantOrderNo":        {},
	"currency":               {},
	"payMethod":              {},
	"status":                 {},
	"startTime":              {},
	"endTime":                {},
	"page":                   {},
	"pageSize":               {},
	"consentNo":              {},
	"subscriptionPaymentNo":  {},
	"planNo":                 {},
	"merchantPlanNo":         {},
	"merchantCustomerNo":     {},
	"subscriptionNo":         {},
	"merchantSubscriptionNo": {},
	"invoiceNo":              {},
}
