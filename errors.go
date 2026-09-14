package joogopay

import (
	"errors"
	"fmt"
)

// Machine-readable error identifiers from the gateway's envelope "msg" field.
// Open set; treat unknown values as opaque strings.
const (
	MsgUnauthorized        = "UNAUTHORIZED"
	MsgInvalidField        = "INVALID_FIELD"
	MsgUnsupportedCurrency = "UNSUPPORTED_CURRENCY"
	MsgUnsupportedMethod   = "UNSUPPORTED_METHOD"
	MsgInsufficientBalance = "INSUFFICIENT_BALANCE"
	MsgMethodNotEnabled    = "METHOD_NOT_ENABLED"
	MsgOrderNotFound       = "ORDER_NOT_FOUND"
	MsgIdempotencyConflict = "IDEMPOTENCY_CONFLICT"
	MsgRateLimited         = "RATE_LIMITED"
	MsgServiceUnavailable  = "SERVICE_UNAVAILABLE"
	MsgInternalError       = "INTERNAL_ERROR"
	MsgOrderRejected       = "ORDER_REJECTED"
	MsgChannelError        = "CHANNEL_ERROR"
	MsgChannelBusy         = "CHANNEL_BUSY"
)

// APIError is the typed business error decoded from a non-success envelope.
type APIError struct {
	HTTPStatus int
	Code       int
	Msg        string
	Message    string
	TraceID    string
	RawBody    []byte
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("sdk: http=%d code=%d msg=%s traceId=%s message=%s",
			e.HTTPStatus, e.Code, e.Msg, e.TraceID, e.Message)
	}
	return fmt.Sprintf("sdk: http=%d code=%d msg=%s traceId=%s",
		e.HTTPStatus, e.Code, e.Msg, e.TraceID)
}

func AsAPIError(err error) (*APIError, bool) {
	var e *APIError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// ResponseError is returned when the server returned a non-envelope body
// (an HTML error page or plain-text 502 from the gateway or CDN); distinct from *APIError.
type ResponseError struct {
	HTTPStatus int
	RawBody    []byte
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("sdk: http=%d response is not a valid envelope JSON", e.HTTPStatus)
}

// ErrInvalidRequest is the parent of every error raised before a request
// leaves the process.
//
// A single errors.Is(err, ErrInvalidRequest) separates two failures with
// opposite handling: a match means the request was never sent and the
// upstream cannot have accepted it, so the caller may safely mark it failed;
// anything else (network, timeout, non-envelope response) leaves the outcome
// unknown, and a payout is never marked failed on it. Treating "never sent"
// as "unknown" strands a payout in pending, waiting for an upstream order
// that does not exist: no query and no webhook will ever settle it.
//
// Every error raised before sending wraps this sentinel with %w; an unwrapped
// one is silently classified by integrators as an unknown outcome.
var ErrInvalidRequest = errors.New("sdk: invalid request")

var (
	ErrMissingBaseURL   = errors.New("sdk: Config.BaseURL is required")
	ErrInvalidBaseURL   = errors.New("sdk: Config.BaseURL must be an absolute origin URL")
	ErrMissingAccessKey = errors.New("sdk: Config.AccessKey is required")

	ErrNilRequest        = fmt.Errorf("%w: request is nil", ErrInvalidRequest)
	ErrInvalidPathParam  = fmt.Errorf("%w: invalid path parameter", ErrInvalidRequest)
	ErrInvalidQueryParam = fmt.Errorf("%w: invalid query parameter", ErrInvalidRequest)
	ErrInvalidExtraField = fmt.Errorf("%w: invalid extra field name", ErrInvalidRequest)
	ErrResponseTooLarge  = errors.New("sdk: response body exceeds MaxResponseBytes")

	// ErrTransport wraps failures after the request left the process (connect,
	// timeout, read). The outcome is unknown: query the order before retrying.
	ErrTransport = errors.New("sdk: transport")

	ErrInvalidWebhookBody = errors.New("sdk: invalid webhook body")

	ErrMissingMethodCode      = fmt.Errorf("%w: paymentMethod.code is required", ErrInvalidRequest)
	ErrConflictingMethodExtra = fmt.Errorf("%w: only one method extra may be set", ErrInvalidRequest)
	ErrMethodExtraMismatch    = fmt.Errorf("%w: method extra does not match code", ErrInvalidRequest)
	ErrMethodNotAvailable     = fmt.Errorf("%w: method is not available for this currency", ErrInvalidRequest)
	ErrMissingRequiredField   = fmt.Errorf("%w: required field is empty", ErrInvalidRequest)
	ErrInvalidAmount          = fmt.Errorf("%w: amount must be a positive decimal string", ErrInvalidRequest)
	ErrInvalidWebhookURL      = fmt.Errorf("%w: webhookUrl must be an absolute https URL", ErrInvalidRequest)

	ErrMissingMerchantPrivateKey        = errors.New("sdk: Config.MerchantPrivateKeyBase64 is required")
	ErrMissingPlatformBodyKeyID         = errors.New("sdk: Config.PlatformBodyKeyID is required")
	ErrMissingPlatformBodyPublicKey     = errors.New("sdk: Config.PlatformBodyPublicKeyBase64 is required")
	ErrMissingPlatformWebhookPublicKeys = errors.New("sdk: Config.PlatformWebhookPublicKeys is required")
	ErrInvalidMerchantPrivateKey        = errors.New("sdk: invalid merchant Ed25519 private key")
	ErrInvalidPlatformBodyPublicKey     = errors.New("sdk: invalid platform X25519 public key")
	ErrInvalidPlatformWebhookPublicKey  = errors.New("sdk: invalid platform webhook Ed25519 public key")
	ErrInvalidIdempotencyKey            = fmt.Errorf("%w: invalid Idempotency-Key", ErrInvalidRequest)
	ErrWebhookPlatformKeyNotFound       = errors.New("sdk: webhook platform key not found")
	ErrWebhookEventIDMismatch           = errors.New("sdk: webhook event id mismatch")
	ErrSignedPostQueryNotAllowed        = fmt.Errorf("%w: signed POST query is not allowed", ErrInvalidRequest)
	ErrSignedGetBodyNotAllowed          = fmt.Errorf("%w: signed GET body is not allowed", ErrInvalidRequest)
	ErrSignedMethodNotSupported         = fmt.Errorf("%w: signed method is not supported", ErrInvalidRequest)
)
