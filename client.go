package joogopay

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
	merchantrequest "github.com/joogopay/sdk-go/internal/merchantrequest"
)

type RequestOption func(*requestOptions)

type requestOptions struct {
	idempotencyKey string
}

func WithIdempotencyKey(key string) RequestOption {
	return func(opts *requestOptions) {
		opts.idempotencyKey = key
	}
}

func applyRequestOptions(opts []RequestOption) requestOptions {
	var out requestOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}

func (c *Client) CreatePayment(ctx context.Context, req *CreatePaymentReq, opts ...RequestOption) (*PaymentOrder, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	if err := validateCreateCommon(req.MerchantOrderNo, req.Currency, req.Amount, req.WebhookUrl); err != nil {
		return nil, err
	}
	if err := validatePaymentMethod(req.Currency, req.PaymentMethod); err != nil {
		return nil, err
	}
	var out PaymentOrder
	if err := c.doWrite(ctx, "/api/v1/payments", req, &out, opts...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreatePayout(ctx context.Context, req *CreatePayoutReq, opts ...RequestOption) (*PayoutOrder, error) {
	if req == nil {
		return nil, ErrNilRequest
	}
	if err := validateCreateCommon(req.MerchantOrderNo, req.Currency, req.Amount, req.WebhookUrl); err != nil {
		return nil, err
	}
	if err := validatePayoutMethod(req.Currency, req.PayoutMethod); err != nil {
		return nil, err
	}
	var out PayoutOrder
	if err := c.doWrite(ctx, "/api/v1/payouts", req, &out, opts...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) QueryPaymentByOrderNo(ctx context.Context, orderNo string) (*PaymentOrder, error) {
	var out PaymentOrder
	if err := c.doRead(ctx, "/api/v1/payments", url.Values{"orderNo": {orderNo}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) QueryPaymentByMerchantOrderNo(ctx context.Context, merchantOrderNo string) (*PaymentOrder, error) {
	var out PaymentOrder
	if err := c.doRead(ctx, "/api/v1/payments", url.Values{"merchantOrderNo": {merchantOrderNo}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) QueryPayoutByOrderNo(ctx context.Context, orderNo string) (*PayoutOrder, error) {
	var out PayoutOrder
	if err := c.doRead(ctx, "/api/v1/payouts", url.Values{"orderNo": {orderNo}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) QueryPayoutByMerchantOrderNo(ctx context.Context, merchantOrderNo string) (*PayoutOrder, error) {
	var out PayoutOrder
	if err := c.doRead(ctx, "/api/v1/payouts", url.Values{"merchantOrderNo": {merchantOrderNo}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPayoutReceipt(ctx context.Context, orderNo string) (*PayoutReceipt, error) {
	orderNo, err := validatePathOrderNo(orderNo)
	if err != nil {
		return nil, err
	}
	var out PayoutReceipt
	path := "/api/v1/payouts/" + url.PathEscape(orderNo) + "/receipt"
	if err := c.doRead(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetBalance(ctx context.Context, currency string) (*Balance, error) {
	var out Balance
	if err := c.doRead(ctx, "/api/v1/balances", url.Values{"currency": {currency}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetUSDRate(ctx context.Context, currency, payMethod string) (*USDRate, error) {
	var out USDRate
	query := url.Values{"currency": {currency}, "payMethod": {payMethod}}
	if err := c.doRead(ctx, "/api/v1/usd-rates", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doWrite(ctx context.Context, path string, reqBody any, out any, opts ...RequestOption) error {
	plain, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("%w: marshal request body: %w", ErrInvalidRequest, err)
	}
	idempotencyKey := strings.TrimSpace(applyRequestOptions(opts).idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey, err = newUUIDv4()
		if err != nil {
			return fmt.Errorf("%w: new idempotency key: %w", ErrInvalidRequest, err)
		}
	}
	if err := merchantauth.ValidateIdempotencyKey(idempotencyKey); err != nil {
		return ErrInvalidIdempotencyKey
	}
	nonce, err := newUUIDv4()
	if err != nil {
		return fmt.Errorf("%w: new nonce: %w", ErrInvalidRequest, err)
	}
	fullURL := c.baseURL + path
	built, err := merchantrequest.BuildWrite(merchantrequest.BuildRequest{
		AccessKey:      c.accessKey,
		IdempotencyKey: idempotencyKey,
		BodyKeyID:      c.platformBodyKeyID,
		BodyPublicKey:  c.platformBodyPublicKey,
		Signer:         c.merchantSigner(),
		EndpointURL:    fullURL,
		Body:           plain,
		Now:            c.now(),
		Nonce:          nonce,
	})
	if err != nil {
		return fmt.Errorf("%w: build signed request: %w", ErrInvalidRequest, err)
	}
	return c.send(ctx, fullURL, built.Method, built.Body, built.Headers, out)
}

func (c *Client) doRead(ctx context.Context, path string, query url.Values, out any) error {
	for _, values := range query {
		for _, v := range values {
			if strings.TrimSpace(v) == "" {
				return ErrInvalidQueryParam
			}
		}
	}
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	nonce, err := newUUIDv4()
	if err != nil {
		return fmt.Errorf("%w: new nonce: %w", ErrInvalidRequest, err)
	}
	built, err := merchantrequest.BuildRead(merchantrequest.BuildRequest{
		AccessKey:   c.accessKey,
		Signer:      c.merchantSigner(),
		EndpointURL: fullURL,
		Now:         c.now(),
		Nonce:       nonce,
	})
	if err != nil {
		return fmt.Errorf("%w: build signed request: %w", ErrInvalidRequest, err)
	}
	return c.send(ctx, fullURL, built.Method, nil, built.Headers, out)
}

// doPublicGet issues an unsigned GET to a public endpoint such as checkout.
func (c *Client) doPublicGet(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return c.send(ctx, endpoint, http.MethodGet, nil, nil, out)
}

// doPublicPost issues an unsigned, unencrypted POST to a public endpoint.
// Only the hosted-checkout group (submitTradeNo / addExtraInfo) is reachable this
// way: those routes are unauthenticated, so the body goes out as plain JSON.
func (c *Client) doPublicPost(ctx context.Context, path string, reqBody any, out any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("%w: marshal request body: %w", ErrInvalidRequest, err)
	}
	return c.send(ctx, c.baseURL+path, http.MethodPost, body,
		map[string]string{"Content-Type": "application/json"}, out)
}

func (c *Client) send(ctx context.Context, endpoint string, method string, body []byte, headers map[string]string, out any) error {
	if method == http.MethodGet && len(body) > 0 {
		return ErrSignedGetBodyNotAllowed
	}
	if method != http.MethodPost && method != http.MethodGet {
		return ErrSignedMethodNotSupported
	}
	if method == http.MethodPost {
		u, err := url.Parse(endpoint)
		if err != nil || u.RawQuery != "" {
			return ErrSignedPostQueryNotAllowed
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: build request: %w", ErrInvalidRequest, err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", c.acceptLanguage)
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: send request: %w", ErrTransport, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return c.decodeResponse(resp, out)
}

func (c *Client) decodeResponse(resp *http.Response, out any) error {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("%w: read response: %w", ErrTransport, err)
	}
	if int64(len(respBody)) > c.maxResponseBytes {
		return ErrResponseTooLarge
	}
	var env envelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return &ResponseError{HTTPStatus: resp.StatusCode, RawBody: respBody}
	}
	if resp.StatusCode != http.StatusOK || env.Code != http.StatusOK {
		return &APIError{
			HTTPStatus: resp.StatusCode,
			Code:       env.Code,
			Msg:        env.Msg,
			Message:    env.dataMessage(),
			TraceID:    env.TraceID,
			RawBody:    respBody,
		}
	}
	// A success envelope always carries the business object. Returning the zero
	// value instead would hand back an order with an empty orderNo that the
	// caller would record as a successful payout.
	if out != nil {
		if len(env.Data) == 0 || string(env.Data) == "null" {
			return &ResponseError{HTTPStatus: resp.StatusCode, RawBody: respBody}
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return &ResponseError{HTTPStatus: resp.StatusCode, RawBody: respBody}
		}
	}
	return nil
}

func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
