package joogopay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
)

type arsWireTransport func(*http.Request) (*http.Response, error)

func (f arsWireTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func arsWireCreate(c *Client, method any, opts ...RequestOption) error {
	switch m := method.(type) {
	case PaymentMethod:
		_, err := c.CreatePayment(context.Background(), &CreatePaymentReq{
			MerchantOrderNo: "ars-wire-payment-001", Currency: CurrencyARS, Amount: "1000.00",
			PaymentMethod: m, WebhookUrl: "https://merchant.example.com/webhook",
		}, opts...)
		return err
	case PayoutMethod:
		_, err := c.CreatePayout(context.Background(), &CreatePayoutReq{
			MerchantOrderNo: "ars-wire-payout-001", Currency: CurrencyARS, Amount: "1000.00",
			PayoutMethod: m, WebhookUrl: "https://merchant.example.com/webhook",
		}, opts...)
		return err
	default:
		panic("unsupported ARS wire fixture method")
	}
}

func TestARSWireExplicitExtraMatchesTypedRequest(t *testing.T) {
	for _, name := range []string{MethodCodeBankTransfer, MethodCodeCVU, MethodCodeQRIS, "payout-CBU", "payout-CVU"} {
		t.Run(name, func(t *testing.T) {
			// An independently spelled map catches field-name drift between typed and dynamic callers.
			extra := map[string]any{
				"firstName": "Ana", "lastName": "Perez", "email": "ana@example.com",
				"documentType": "DNI", "documentNumber": "30123456",
			}
			var typed, dynamic any
			if accountType, payout := strings.CutPrefix(name, "payout-"); payout {
				extra["phone"], extra["address"] = "1123456789", "Av Example 123"
				extra["accountType"], extra["accountNo"] = accountType, "0000003100012345678901"
				typed = arsPayoutMethod(accountType)
				m := PayoutMethod{Code: MethodCodeBankTransfer}
				if err := m.SetExtra("bankTransfer", extra); err != nil {
					t.Fatal(err)
				}
				dynamic = m
			} else {
				branch := map[string]string{MethodCodeBankTransfer: "bankTransfer", MethodCodeCVU: "cvu", MethodCodeQRIS: "qris"}[name]
				if name != MethodCodeBankTransfer {
					extra["phone"] = "1123456789"
				}
				typed = arsPaymentMethod(name)
				m := PaymentMethod{Code: name}
				if err := m.SetExtra(branch, extra); err != nil {
					t.Fatal(err)
				}
				dynamic = m
			}

			k, cap := newTestKeys(t), &capture{}
			srv := verifyingServer(t, k, `{}`, cap)
			defer srv.Close()
			c := k.client(t, srv.URL, srv.Client())
			var bodies [2]map[string]any
			for i, method := range []any{typed, dynamic} {
				if err := arsWireCreate(c, method); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(cap.openedBody, &bodies[i]); err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(bodies[0], bodies[1]) {
				t.Fatal("typed and explicit extra differ after server signature verification and decryption")
			}
		})
	}
}

func TestARSWireQuerySignsEncodedOrderIdentifier(t *testing.T) {
	cases := []struct {
		name, path, query string
		call              func(*Client) error
	}{
		{"payment/orderNo", "/api/v1/payments", "orderNo=FPARS0001", func(c *Client) error {
			_, err := c.QueryPaymentByOrderNo(context.Background(), "FPARS0001")
			return err
		}},
		{"payment/merchantOrderNo", "/api/v1/payments", "merchantOrderNo=ars%2Bpedido%2F2026%3Fref%3Dni%C3%B1o%26num%3D01", func(c *Client) error {
			_, err := c.QueryPaymentByMerchantOrderNo(context.Background(), "ars+pedido/2026?ref=niño&num=01")
			return err
		}},
		{"payout/orderNo", "/api/v1/payouts", "orderNo=FOARS0001", func(c *Client) error {
			_, err := c.QueryPayoutByOrderNo(context.Background(), "FOARS0001")
			return err
		}},
		{"payout/merchantOrderNo", "/api/v1/payouts", "merchantOrderNo=ars%2Bpedido%2F2026%3Fref%3Dni%C3%B1o%26num%3D01", func(c *Client) error {
			_, err := c.QueryPayoutByMerchantOrderNo(context.Background(), "ars+pedido/2026?ref=niño&num=01")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, cap := newTestKeys(t), &capture{}
			srv := verifyingServer(t, k, `{"currency":"ARS","amount":"1000.00"}`, cap)
			defer srv.Close()
			hc := srv.Client()
			transport := hc.Transport
			hc.Transport = arsWireTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet || r.URL.Path != tc.path {
					t.Errorf("query route: got %s %s", r.Method, r.URL.Path)
				}
				return transport.RoundTrip(r)
			})
			if err := tc.call(k.client(t, srv.URL, hc)); err != nil {
				t.Fatal(err)
			}
			// The server verifies the signature over the actual RawQuery, including escapes.
			if cap.query != tc.query {
				t.Fatalf("raw query: got %q, want %q", cap.query, tc.query)
			}
		})
	}
}

func TestARSWireExplicitRetryPreservesBusinessIdentity(t *testing.T) {
	cases := []struct {
		name   string
		method any
	}{
		{"payment", arsPaymentMethod(MethodCodeCVU)},
		{"payout", arsPayoutMethod("CVU")},
	}
	for _, address := range []string{"missing", "null", "empty"} {
		method := arsPayoutMethod("CVU")
		method.BankTransfer.Address = ""
		if address != "missing" {
			body, err := json.Marshal(method.BankTransfer)
			if err != nil {
				t.Fatal(err)
			}
			var extra map[string]any
			if err := json.Unmarshal(body, &extra); err != nil {
				t.Fatal(err)
			}
			extra["address"] = nil
			if address == "empty" {
				extra["address"] = ""
			}
			if err := method.SetExtra("bankTransfer", extra); err != nil {
				t.Fatal(err)
			}
		}
		cases = append(cases, struct {
			name   string
			method any
		}{"payout-address-" + address, method})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, cap := newTestKeys(t), &capture{}
			srv := verifyingServer(t, k, `{}`, cap)
			defer srv.Close()
			type attempt struct {
				nonce, idempotencyKey string
				plain                 []byte
			}
			var attempts []attempt
			hc := srv.Client()
			transport := hc.Transport
			hc.Transport = arsWireTransport(func(r *http.Request) (*http.Response, error) {
				params, err := merchantauth.ParseMerchantWriteSignatureInput(r.Header.Get(merchantauth.HeaderSignatureInput))
				if err != nil {
					return nil, err
				}
				resp, err := transport.RoundTrip(r)
				if err != nil {
					return nil, err
				}
				attempts = append(attempts, attempt{
					nonce: params.Nonce, idempotencyKey: r.Header.Get(merchantauth.HeaderIdempotencyKey),
					plain: append([]byte(nil), cap.openedBody...),
				})
				if len(attempts) == 1 {
					// Model a gateway 502 after the fake platform already accepted the request.
					_ = resp.Body.Close()
					resp.StatusCode = http.StatusBadGateway
					resp.Body = io.NopCloser(strings.NewReader("gateway temporarily unavailable"))
				}
				return resp, nil
			})
			c := k.client(t, srv.URL, hc)
			const idempotencyKey = "018fb9b4-95f3-4a47-8f08-27466f7d4c1d"
			err := arsWireCreate(c, tc.method, WithIdempotencyKey(idempotencyKey))
			var responseErr *ResponseError
			if !errors.As(err, &responseErr) || responseErr.HTTPStatus != http.StatusBadGateway || errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("uncertain response was misclassified: %v", err)
			}
			if len(attempts) != 1 {
				t.Fatalf("write automatically retried: %d attempts", len(attempts))
			}
			if err := arsWireCreate(c, tc.method, WithIdempotencyKey(idempotencyKey)); err != nil {
				t.Fatal(err)
			}
			if len(attempts) != 2 {
				t.Fatalf("explicit retry: got %d attempts", len(attempts))
			}
			for _, a := range attempts {
				if a.idempotencyKey != idempotencyKey {
					t.Fatal("caller-provided idempotency key changed")
				}
			}
			if attempts[0].nonce == attempts[1].nonce {
				t.Fatal("retry reused the request nonce")
			}
			if !bytes.Equal(attempts[0].plain, attempts[1].plain) {
				t.Fatal("retry changed the merchant order number or business body")
			}
		})
	}
}

func TestARSWirePaymentReturnURL(t *testing.T) {
	for _, tc := range []struct{ name, returnURL string }{
		{"omitted", ""},
		{"encoded_query_and_fragment", "https://merchant.example.com/ars/return?reference=pedido%2B001&label=Ana+P%C3%A9rez#receipt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, cap := newTestKeys(t), &capture{}
			srv := verifyingServer(t, k, `{}`, cap)
			defer srv.Close()
			c := k.client(t, srv.URL, srv.Client())
			_, err := c.CreatePayment(context.Background(), &CreatePaymentReq{
				MerchantOrderNo: "ars-wire-return-001", Currency: CurrencyARS, Amount: "1000.00",
				PaymentMethod: arsPaymentMethod(MethodCodeCVU), ReturnUrl: tc.returnURL,
				WebhookUrl: "https://merchant.example.com/webhook",
			})
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]any
			if err := json.Unmarshal(cap.openedBody, &body); err != nil {
				t.Fatal(err)
			}
			got, present := body["returnUrl"]
			if tc.returnURL == "" {
				if present {
					t.Fatal("omitted returnUrl was serialized or invented")
				}
			} else if got != tc.returnURL {
				t.Fatal("returnUrl changed during request signing and encryption")
			}
		})
	}
}
