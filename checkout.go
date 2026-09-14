package joogopay

import (
	"context"
	"net/url"
	"strings"
)

type CheckoutParams struct {
	PayContent string `json:"payContent,omitempty"`
	// Reusable indicates the payment content can stay visible after a successful payment.
	Reusable bool `json:"reusable,omitempty"`
}

type PaymentCheckout struct {
	OrderNo     string         `json:"orderNo"`
	OrderStatus int64          `json:"orderStatus"`
	Status      string         `json:"status"`
	Amount      string         `json:"amount"`
	Currency    string         `json:"currency"`
	PayMethod   string         `json:"payMethod"`
	Attach      string         `json:"attach"`
	ReturnUrl   string         `json:"returnUrl,omitempty"`
	Params      CheckoutParams `json:"params"`
	CreateTime  int64          `json:"createTime"`
	UpdateTime  int64          `json:"updateTime"`
}

// GetPaymentCheckout is unsigned and unencrypted; /api/v1/payment/checkout is a public
// endpoint used by the H5 checkout page and callable by the merchant backend for reconciliation.
func (c *Client) GetPaymentCheckout(ctx context.Context, orderNo string) (*PaymentCheckout, error) {
	if orderNo = strings.TrimSpace(orderNo); orderNo == "" || strings.ContainsAny(orderNo, "/?#") {
		return nil, ErrInvalidPathParam
	}
	var out PaymentCheckout
	if err := c.doPublicGet(ctx, "/api/v1/payment/checkout", url.Values{"orderNo": {orderNo}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SubmitTradeNoResult is the answer to SubmitPaymentTradeNo.
//
// A nil error does not mean the platform accepted the call: the endpoint
// answers HTTP 200 with envelope code 200 even when it refuses, and the outcome
// is carried by Status: 1 accepted, 0 refused with Message giving the reason,
// so a nil error alone does not mean the submission was accepted.
type SubmitTradeNoResult struct {
	Status int64 `json:"status"`
	// OrderStatus is the checkout-facing order status (CREATED / PENDING / …).
	OrderStatus string `json:"orderStatus,omitempty"`
	Message     string `json:"message,omitempty"`
}

// Ok reports whether the platform accepted the submission.
func (r *SubmitTradeNoResult) Ok() bool { return r != nil && r.Status == 1 }

// AddExtraInfoResult is the answer to AddPaymentExtraInfo; like
// SubmitTradeNoResult it reports acceptance through Status, not through the error.
type AddExtraInfoResult struct {
	Status int64 `json:"status"`
	// OrderStatus is the checkout-facing order status (CREATED / PENDING / …).
	OrderStatus string `json:"orderStatus,omitempty"`
	// PaymentUrl is set once the platform placed the order upstream; the payer
	// is redirected there.
	PaymentUrl string `json:"paymentUrl,omitempty"`
	Message    string `json:"message,omitempty"`
}

// Ok reports whether the platform accepted the extra info.
func (r *AddExtraInfoResult) Ok() bool { return r != nil && r.Status == 1 }

type submitTradeNoReq struct {
	OrderNo string `json:"orderNo"`
	TradeNo string `json:"tradeNo"`
}

type addExtraInfoReq struct {
	OrderNo   string            `json:"orderNo"`
	PayMethod string            `json:"payMethod,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

// SubmitPaymentTradeNo reports the upstream trade number (UTR) a payer typed
// into the H5 checkout page, so the platform can match the transfer to the order.
// Like GetPaymentCheckout it is unsigned and unencrypted, plain JSON both ways:
// the endpoint serves the merchant's own H5 checkout page, which has no access
// to the merchant private key. A nil error is not acceptance; see SubmitTradeNoResult.
func (c *Client) SubmitPaymentTradeNo(ctx context.Context, orderNo, tradeNo string) (*SubmitTradeNoResult, error) {
	orderNo, tradeNo = strings.TrimSpace(orderNo), strings.TrimSpace(tradeNo)
	if orderNo == "" || tradeNo == "" {
		return nil, ErrInvalidQueryParam
	}
	var out SubmitTradeNoResult
	if err := c.doPublicPost(ctx, "/api/v1/payment/submitTradeNo",
		&submitTradeNoReq{OrderNo: orderNo, TradeNo: tradeNo}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddPaymentExtraInfo completes a "create first, fill in later" order: the
// merchant may create the payment without payer details, and the H5 checkout
// page supplies them here, which is what triggers the real upstream order.
// payMethod and extra may be empty when the order already carries them.
// Unsigned and unencrypted like SubmitPaymentTradeNo; a nil error is not
// acceptance, see AddExtraInfoResult.
func (c *Client) AddPaymentExtraInfo(ctx context.Context, orderNo, payMethod string,
	extra map[string]string) (*AddExtraInfoResult, error) {
	if orderNo = strings.TrimSpace(orderNo); orderNo == "" {
		return nil, ErrInvalidQueryParam
	}
	var out AddExtraInfoResult
	if err := c.doPublicPost(ctx, "/api/v1/payment/addExtraInfo",
		&addExtraInfoReq{OrderNo: orderNo, PayMethod: strings.TrimSpace(payMethod), Extra: extra},
		&out); err != nil {
		return nil, err
	}
	return &out, nil
}
