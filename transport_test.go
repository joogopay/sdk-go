package joogopay

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type failingTransport struct{ err error }

func (f failingTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

// A body that cannot be encoded never leaves the process, so it is an invalid
// request, not a transport failure.
func TestUnencodableRequestIsInvalidRequest(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com", &http.Client{Transport: failingTransport{err: errors.New("must not be reached")}})
	m := PaymentMethod{Code: "PIX"}
	if err := m.SetExtra("pix", map[string]any{"bad": make(chan int)}); err != nil {
		t.Fatal(err)
	}
	_, err := c.CreatePayment(context.Background(), &CreatePaymentReq{
		MerchantOrderNo: "M1", Currency: CurrencyBRL, Amount: "1.00", PaymentMethod: m,
		WebhookUrl: "https://merchant.example/webhook",
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("want ErrInvalidRequest, got %v", err)
	}
	if errors.Is(err, ErrTransport) {
		t.Fatalf("encoding failure must not look like a transport failure: %v", err)
	}
}

func TestTransportFailureIsNotInvalidRequest(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com", &http.Client{Transport: failingTransport{err: errors.New("connect: connection refused")}})
	_, err := c.GetBalance(context.Background(), "BRL")
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("want ErrTransport, got %v", err)
	}
	if errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("transport failure must not be an invalid request: %v", err)
	}
}

// Every failure on a send path is either "never sent" or "outcome unknown"; a
// merchant that cannot tell them apart has to treat a local rejection as
// indeterminate and leaves a payout pending on an order that was never created.
func TestSendPathErrorsAreClassified(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com", &http.Client{Transport: failingTransport{err: errors.New("connect: connection refused")}})
	ctx := context.Background()
	oversized := strings.Repeat("x", 1<<20)

	cases := []struct {
		name string
		call func() error
	}{
		{"body over the envelope limit", func() error {
			_, err := c.CreatePayment(ctx, &CreatePaymentReq{
				MerchantOrderNo: "M1", Currency: CurrencyBRL, Amount: "1.00",
				PaymentMethod: PaymentMethod{Code: "PIX", Pix: &PaymentPixExtra{PayerName: oversized}},
				WebhookUrl:    "https://merchant.example/webhook",
			})
			return err
		}},
		{"transport refuses", func() error { _, err := c.GetBalance(ctx, "BRL"); return err }},
		{"blank path parameter", func() error { _, err := c.QueryPaymentByOrderNo(ctx, " "); return err }},
		{"missing required field", func() error {
			_, err := c.CreatePayout(ctx, &CreatePayoutReq{Currency: CurrencyBRL, Amount: "1.00"})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("want an error")
			}
			invalid, transport := errors.Is(err, ErrInvalidRequest), errors.Is(err, ErrTransport)
			if invalid == transport {
				t.Fatalf("error must be exactly one of ErrInvalidRequest / ErrTransport, got invalid=%v transport=%v: %v", invalid, transport, err)
			}
		})
	}
}
