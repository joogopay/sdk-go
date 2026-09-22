package joogopay

import (
	"encoding/json"
	"fmt"
	"net/http"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
)

const (
	WebhookOrderTypePayment = "PAYMENT"
	WebhookOrderTypePayout  = "PAYOUT"
)

type PaymentWebhook struct {
	EventID         string          `json:"eventId"`
	OrderType       string          `json:"orderType"`
	OrderNo         string          `json:"orderNo"`
	MerchantOrderNo string          `json:"merchantOrderNo"`
	Status          string          `json:"status"`
	Currency        string          `json:"currency"`
	Amount          string          `json:"amount"`
	PaidAmount      string          `json:"paidAmount,omitempty"`
	Payer           *PaymentPayer   `json:"payer,omitempty"`
	ChannelTradeNo  string          `json:"channelTradeNo,omitempty"`
	Attach          string          `json:"attach,omitempty"`
	Failure         *WebhookFailure `json:"failure,omitempty"`
}

type PayoutWebhook struct {
	EventID         string          `json:"eventId"`
	OrderType       string          `json:"orderType"`
	OrderNo         string          `json:"orderNo"`
	MerchantOrderNo string          `json:"merchantOrderNo"`
	Status          string          `json:"status"`
	Currency        string          `json:"currency"`
	Amount          string          `json:"amount"`
	ChannelTradeNo  string          `json:"channelTradeNo,omitempty"`
	Attach          string          `json:"attach,omitempty"`
	Failure         *WebhookFailure `json:"failure,omitempty"`
}

type WebhookFailure struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Message string `json:"message"`
}

func (c *Client) VerifyWebhook(r *http.Request) ([]byte, error) {
	if r == nil {
		return nil, ErrNilRequest
	}
	if err := merchantauth.VerifyWebhookRequestShape(r); err != nil {
		return nil, err
	}
	body, err := merchantauth.ReadBody(r)
	if err != nil {
		return nil, err
	}
	if !merchantauth.VerifyContentDigest(body, r.Header.Get(merchantauth.HeaderContentDigest)) {
		return nil, merchantauth.ErrInvalidSignature
	}
	eventID := r.Header.Get(merchantauth.HeaderWebhookEventID)
	if err := merchantauth.ValidateWebhookEventID(eventID); err != nil {
		return nil, err
	}
	var head struct {
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(body, &head); err != nil || head.EventID != eventID {
		return nil, ErrWebhookEventIDMismatch
	}
	params, err := merchantauth.ParsePlatformSignatureInput(r.Header.Get(merchantauth.HeaderSignatureInput))
	if err != nil {
		return nil, err
	}
	if err := merchantauth.ValidateSignatureParams(params, c.now()); err != nil {
		return nil, err
	}
	publicKey, ok := c.platformWebhookPublicKeys[params.KeyID]
	if !ok {
		return nil, ErrWebhookPlatformKeyNotFound
	}
	signature, err := merchantauth.ParseSignature(r.Header.Get(merchantauth.HeaderSignature), merchantauth.SignatureLabelPlatform)
	if err != nil {
		return nil, err
	}
	base, err := merchantauth.SignatureBase(r, params)
	if err != nil {
		return nil, err
	}
	if err := merchantauth.VerifyEd25519(publicKey, base, signature); err != nil {
		return nil, err
	}
	return body, nil
}

func (c *Client) ParsePaymentWebhook(r *http.Request) (*PaymentWebhook, error) {
	body, err := c.VerifyWebhook(r)
	if err != nil {
		return nil, err
	}
	var payload PaymentWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidWebhookBody, err)
	}
	if payload.EventID == "" || payload.OrderType != WebhookOrderTypePayment || payload.OrderNo == "" || payload.MerchantOrderNo == "" || payload.Status == "" {
		return nil, ErrInvalidWebhookBody
	}
	return &payload, nil
}

func (c *Client) ParsePayoutWebhook(r *http.Request) (*PayoutWebhook, error) {
	body, err := c.VerifyWebhook(r)
	if err != nil {
		return nil, err
	}
	var payload PayoutWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidWebhookBody, err)
	}
	if payload.EventID == "" || payload.OrderType != WebhookOrderTypePayout || payload.OrderNo == "" || payload.MerchantOrderNo == "" || payload.Status == "" {
		return nil, ErrInvalidWebhookBody
	}
	return &payload, nil
}
