package joogopay

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
)

func TestCreatePaymentSignsAndSeals(t *testing.T) {
	k := newTestKeys(t)
	cap := &capture{}
	srv := verifyingServer(t, k, `{"orderNo":"P1","merchantOrderNo":"M1","status":"PENDING","currency":"BRL","amount":"100.50","paidAmount":"0.00","paymentMethod":"PIX","action":{}}`, cap)
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())

	order, err := c.CreatePayment(context.Background(), &CreatePaymentReq{
		MerchantOrderNo: "M1", Currency: "BRL", Amount: "100.50",
		PaymentMethod: PaymentMethod{Code: "PIX"}, WebhookUrl: "https://m.example.com/hook",
	}, WithIdempotencyKey("018fb9b4-95f3-4a47-8f08-27466f7d4c1d"))
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if order.OrderNo != "P1" || order.Amount != "100.50" {
		t.Fatalf("unexpected order: %+v", order)
	}
	if !strings.Contains(string(cap.openedBody), `"amount":"100.50"`) {
		t.Fatalf("sealed body not recovered: %s", cap.openedBody)
	}

	if _, err := c.CreatePayment(context.Background(), nil); err != ErrNilRequest {
		t.Fatalf("nil req: %v", err)
	}
	if _, err := c.CreatePayment(context.Background(), validPaymentReq(), WithIdempotencyKey("bad")); err != ErrInvalidIdempotencyKey {
		t.Fatalf("bad idempotency: %v", err)
	}
}

func TestCreatePayoutAndAutoIdempotency(t *testing.T) {
	k := newTestKeys(t)
	srv := verifyingServer(t, k, `{"orderNo":"PO1","merchantOrderNo":"M2","status":"PENDING","currency":"BRL","amount":"10.00","payoutMethod":"PIX","action":{}}`, nil)
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())
	if _, err := c.CreatePayout(context.Background(), validPayoutReq()); err != nil {
		t.Fatalf("CreatePayout: %v", err)
	}
	if _, err := c.CreatePayout(context.Background(), nil); err != ErrNilRequest {
		t.Fatalf("nil payout: %v", err)
	}
}

func TestReadEndpoints(t *testing.T) {
	k := newTestKeys(t)
	cap := &capture{}
	srv := verifyingServer(t, k, `{"orderNo":"P1","merchantOrderNo":"M1","status":"SUCCEEDED","currency":"BRL","amount":"100.50","paidAmount":"100.50","paymentMethod":"PIX","action":{}}`, cap)
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())
	ctx := context.Background()

	for _, call := range []func() error{
		func() error { _, e := c.QueryPaymentByOrderNo(ctx, "P1"); return e },
		func() error { _, e := c.QueryPaymentByMerchantOrderNo(ctx, "M1"); return e },
		func() error { _, e := c.QueryPayoutByOrderNo(ctx, "PO1"); return e },
		func() error { _, e := c.QueryPayoutByMerchantOrderNo(ctx, "M2"); return e },
		func() error { _, e := c.GetBalance(ctx, "BRL"); return e },
		func() error { _, e := c.GetUSDRate(ctx, "BRL", "PIX"); return e },
		func() error { _, e := c.GetPayoutReceipt(ctx, "PO1"); return e },
	} {
		if err := call(); err != nil {
			t.Fatalf("read call: %v", err)
		}
	}
	if _, err := c.GetPayoutReceipt(ctx, ""); err != ErrInvalidPathParam {
		t.Fatalf("empty receipt orderNo: %v", err)
	}
}

func TestMethodsPropagateError(t *testing.T) {
	k := newTestKeys(t)
	srv := errorServer(t)
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())
	ctx := context.Background()

	calls := []func() error{
		func() error {
			_, e := c.CreatePayment(ctx, validPaymentReq())
			return e
		},
		func() error {
			_, e := c.CreatePayout(ctx, validPayoutReq())
			return e
		},
		func() error { _, e := c.QueryPaymentByOrderNo(ctx, "P1"); return e },
		func() error { _, e := c.QueryPaymentByMerchantOrderNo(ctx, "M1"); return e },
		func() error { _, e := c.QueryPayoutByOrderNo(ctx, "PO1"); return e },
		func() error { _, e := c.QueryPayoutByMerchantOrderNo(ctx, "M2"); return e },
		func() error { _, e := c.GetBalance(ctx, "BRL"); return e },
		func() error { _, e := c.GetUSDRate(ctx, "BRL", "PIX"); return e },
		func() error { _, e := c.GetPayoutReceipt(ctx, "PO1"); return e },
		func() error { _, e := c.GetPaymentCheckout(ctx, "P1"); return e },
	}
	for i, call := range calls {
		if _, ok := AsAPIError(call()); !ok {
			t.Fatalf("call %d: want APIError", i)
		}
	}
}

func TestTransportError(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://127.0.0.1:1")
	ctx := context.Background()
	if _, err := c.CreatePayment(ctx, &CreatePaymentReq{Amount: "1.00"}); err == nil {
		t.Fatal("want transport error (write)")
	}
	if _, err := c.GetBalance(ctx, "BRL"); err == nil {
		t.Fatal("want transport error (read)")
	}
	if _, err := c.GetPaymentCheckout(ctx, "P1"); err == nil {
		t.Fatal("want transport error (public)")
	}
}

func TestSendGuards(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com")
	ctx := context.Background()
	if err := c.send(ctx, "https://x/y", http.MethodGet, []byte("b"), nil, nil); err != ErrSignedGetBodyNotAllowed {
		t.Fatalf("get+body: %v", err)
	}
	if err := c.send(ctx, "https://x/y", http.MethodPut, nil, nil, nil); err != ErrSignedMethodNotSupported {
		t.Fatalf("put: %v", err)
	}
	if err := c.send(ctx, "https://x/y?q=1", http.MethodPost, nil, nil, nil); err != ErrSignedPostQueryNotAllowed {
		t.Fatalf("post+query: %v", err)
	}
}

func TestDecodeResponseBranches(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com")

	mk := func(status int, body string) *http.Response {
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
	}
	err := c.decodeResponse(mk(200, `{"code":40400,"msg":"ORDER_NOT_FOUND","traceId":"t","data":{"message":"not found"}}`), nil)
	if ae, ok := AsAPIError(err); !ok || ae.Code != 40400 || ae.Message != "not found" {
		t.Fatalf("want APIError, got %v", err)
	}
	if _, ok := c.decodeResponse(mk(502, `<html>bad gateway`), nil).(*ResponseError); !ok {
		t.Fatal("want ResponseError")
	}
	c.maxResponseBytes = 4
	if err := c.decodeResponse(mk(200, `{"code":200}`), nil); err != ErrResponseTooLarge {
		t.Fatalf("want too large, got %v", err)
	}
	c.maxResponseBytes = defaultMaxResponseBytes

	var out PaymentOrder
	// A caller that asked for the business object must not receive the zero value:
	// an order with an empty orderNo would be recorded as a successful payout.
	if _, ok := c.decodeResponse(mk(200, `{"code":200,"data":null}`), &out).(*ResponseError); !ok {
		t.Fatal("null data must be a ResponseError")
	}
	if _, ok := c.decodeResponse(mk(200, `{"code":200}`), &out).(*ResponseError); !ok {
		t.Fatal("absent data must be a ResponseError")
	}
	if err := c.decodeResponse(mk(200, `{"code":200,"data":null}`), nil); err != nil {
		t.Fatalf("a caller that wants no data tolerates a null: %v", err)
	}
	if err := c.decodeResponse(mk(200, `{"code":200,"data":{"orderNo":"P1"}}`), &out); err != nil || out.OrderNo != "P1" {
		t.Fatalf("data decode: %v", err)
	}
	if err := c.decodeResponse(mk(200, `{"code":200,"data":123}`), &out); err == nil {
		t.Fatal("want unmarshal error")
	}
	if err := c.decodeResponse(&http.Response{StatusCode: 200, Body: io.NopCloser(errReader{})}, nil); err == nil {
		t.Fatal("want read error")
	}
}

func TestNewUUIDv4(t *testing.T) {
	u, err := newUUIDv4()
	if err != nil {
		t.Fatal(err)
	}
	if !merchantauth.ValidUUIDv4(u) {
		t.Fatalf("invalid uuid: %s", u)
	}
}

func TestReadRejectsEmptyQueryParam(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com")
	ctx := context.Background()
	if _, err := c.QueryPaymentByOrderNo(ctx, ""); err != ErrInvalidQueryParam {
		t.Fatalf("empty orderNo: %v", err)
	}
	if _, err := c.GetBalance(ctx, "  "); err != ErrInvalidQueryParam {
		t.Fatalf("blank currency: %v", err)
	}
	if _, err := c.GetUSDRate(ctx, "BRL", ""); err != ErrInvalidQueryParam {
		t.Fatalf("empty payMethod: %v", err)
	}
}

func TestIdempotencyKeyIsTrimmed(t *testing.T) {
	k := newTestKeys(t)
	cap := &capture{}
	srv := verifyingServer(t, k, `{"orderNo":"P1","merchantOrderNo":"M1","status":"PENDING","currency":"BRL","amount":"1.00","paidAmount":"0.00","paymentMethod":"PIX","action":{}}`, cap)
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())
	_, err := c.CreatePayment(context.Background(), validPaymentReq(),
		WithIdempotencyKey("  018fb9b4-95f3-4a47-8f08-27466f7d4c1d "))
	if err != nil {
		t.Fatalf("trimmed key should be accepted: %v", err)
	}
	if cap.idempotencyKey != "018fb9b4-95f3-4a47-8f08-27466f7d4c1d" {
		t.Fatalf("header not trimmed: %q", cap.idempotencyKey)
	}
}
