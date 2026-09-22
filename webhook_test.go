package joogopay

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
)

func signWebhookAt(t *testing.T, k testKeys, body []byte, eid string, now time.Time) *http.Request {
	t.Helper()
	r, _ := http.NewRequest(http.MethodPost, "https://m.example.com/hook", strings.NewReader(string(body)))
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(merchantauth.HeaderContentDigest, merchantauth.ContentDigestSHA256(body))
	r.Header.Set(merchantauth.HeaderWebhookEventID, eid)
	nonce, _ := newUUIDv4()
	params := merchantauth.NewPlatformSignatureParams(k.webhookKeyID, nonce, now)
	sigInput, err := merchantauth.SignatureInputHeader(params)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set(merchantauth.HeaderSignatureInput, sigInput)
	base, err := merchantauth.SignatureBase(r, params)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := merchantauth.SignEd25519(k.webhookPriv, merchantauth.SignatureLabelPlatform, base)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set(merchantauth.HeaderSignature, sig)
	return r
}

func signWebhook(t *testing.T, k testKeys, body []byte, eid string) *http.Request {
	return signWebhookAt(t, k, body, eid, time.Now())
}

func TestVerifyWebhookSuccessAndParse(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com")
	eid := newEventID()
	body, _ := json.Marshal(PaymentWebhook{EventID: eid, OrderType: WebhookOrderTypePayment, OrderNo: "P1", MerchantOrderNo: "M1", Status: "SUCCEEDED", Currency: "BRL", Amount: "100.50"})

	got, err := c.VerifyWebhook(signWebhook(t, k, body, eid))
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if string(got) != string(body) {
		t.Fatal("body mismatch")
	}

	pw, err := c.ParsePaymentWebhook(signWebhook(t, k, body, eid))
	if err != nil || pw.OrderNo != "P1" {
		t.Fatalf("ParsePaymentWebhook: %v %+v", err, pw)
	}

	pbody, _ := json.Marshal(PayoutWebhook{EventID: eid, OrderType: WebhookOrderTypePayout, OrderNo: "PO1", MerchantOrderNo: "M2", Status: "SUCCEEDED", Currency: "BRL", Amount: "10.00"})
	po, err := c.ParsePayoutWebhook(signWebhook(t, k, pbody, eid))
	if err != nil || po.OrderNo != "PO1" {
		t.Fatalf("ParsePayoutWebhook: %v %+v", err, po)
	}
}

func TestVerifyWebhookRejections(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com")
	eid := newEventID()
	body, _ := json.Marshal(PaymentWebhook{EventID: eid, OrderType: WebhookOrderTypePayment, OrderNo: "P1", MerchantOrderNo: "M1", Status: "SUCCEEDED", Currency: "BRL", Amount: "1.00"})

	if _, err := c.VerifyWebhook(nil); err != ErrNilRequest {
		t.Fatalf("nil: %v", err)
	}
	// tampered body -> digest mismatch
	r := signWebhook(t, k, body, eid)
	r.Body = io.NopCloser(strings.NewReader(`{"eventId":"x"}`))
	if _, err := c.VerifyWebhook(r); err == nil {
		t.Fatal("want digest mismatch")
	}
	// event id header != body eventId
	if _, err := c.VerifyWebhook(signWebhook(t, k, body, newEventID())); err != ErrWebhookEventIDMismatch {
		t.Fatalf("event id mismatch")
	}
	// unknown platform key
	kk := k
	kk.webhookKeyID = "unknown_key"
	if _, err := c.VerifyWebhook(signWebhook(t, kk, body, eid)); err != ErrWebhookPlatformKeyNotFound {
		t.Fatalf("unknown key")
	}
	// wrong signing key -> signature failure
	bad := newTestKeys(t)
	bad.webhookKeyID = k.webhookKeyID
	if _, err := c.VerifyWebhook(signWebhook(t, bad, body, eid)); err == nil {
		t.Fatal("want signature failure")
	}
	// wrong order type for parse
	pbody, _ := json.Marshal(PayoutWebhook{EventID: eid, OrderType: WebhookOrderTypePayout, OrderNo: "PO1", MerchantOrderNo: "M2", Status: "SUCCEEDED"})
	if _, err := c.ParsePaymentWebhook(signWebhook(t, k, pbody, eid)); err != ErrInvalidWebhookBody {
		t.Fatalf("wrong order type: %v", err)
	}
}

func TestVerifyWebhookShapeAndFreshness(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com")
	eid := newEventID()
	body, _ := json.Marshal(PaymentWebhook{EventID: eid, OrderType: WebhookOrderTypePayment, OrderNo: "P1", MerchantOrderNo: "M1", Status: "SUCCEEDED"})

	// missing Signature-Input -> shape rejection
	bad := signWebhook(t, k, body, eid)
	bad.Header.Del(merchantauth.HeaderSignatureInput)
	if _, err := c.VerifyWebhook(bad); err == nil {
		t.Fatal("want shape rejection")
	}
	// invalid event-id format
	badBody, _ := json.Marshal(PaymentWebhook{EventID: "not-an-event-id", OrderType: WebhookOrderTypePayment, OrderNo: "P1", MerchantOrderNo: "M1", Status: "SUCCEEDED"})
	if _, err := c.VerifyWebhook(signWebhook(t, k, badBody, "not-an-event-id")); err == nil {
		t.Fatal("want event-id format rejection")
	}
	// expired signature
	if _, err := c.VerifyWebhook(signWebhookAt(t, k, body, eid, time.Now().Add(-10*time.Minute))); err == nil {
		t.Fatal("want expired rejection")
	}
	// malformed Signature-Input that passes shape but fails parse
	r2 := signWebhook(t, k, body, eid)
	r2.Header.Set(merchantauth.HeaderSignatureInput, "garbage")
	if _, err := c.VerifyWebhook(r2); err == nil {
		t.Fatal("want sig-input parse rejection")
	}
	// valid sig-input/key but corrupt Signature header -> ParseSignature error
	r3 := signWebhook(t, k, body, eid)
	r3.Header.Set(merchantauth.HeaderSignature, "platform=:not-base64:")
	if _, err := c.VerifyWebhook(r3); err == nil {
		t.Fatal("want signature parse rejection")
	}
	// no body -> ReadBody error
	nb, _ := http.NewRequest(http.MethodPost, "https://m/hook", nil)
	nb.Header.Set("Content-Type", "application/json")
	nb.Header.Set(merchantauth.HeaderContentDigest, merchantauth.ContentDigestSHA256(nil))
	nb.Header.Set(merchantauth.HeaderWebhookEventID, eid)
	nonce, _ := newUUIDv4()
	p := merchantauth.NewPlatformSignatureParams(k.webhookKeyID, nonce, time.Now())
	si, _ := merchantauth.SignatureInputHeader(p)
	nb.Header.Set(merchantauth.HeaderSignatureInput, si)
	nb.Header.Set(merchantauth.HeaderSignature, "platform=:AAAA:")
	if _, err := c.VerifyWebhook(nb); err == nil {
		t.Fatal("want body rejection")
	}
}

func TestParseWebhookErrors(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com")
	eid := newEventID()
	// VerifyWebhook error propagates
	if _, err := c.ParsePaymentWebhook(nil); err != ErrNilRequest {
		t.Fatalf("parse payment nil: %v", err)
	}
	if _, err := c.ParsePayoutWebhook(nil); err != ErrNilRequest {
		t.Fatalf("parse payout nil: %v", err)
	}
	// valid signature but incomplete body -> ErrInvalidWebhookBody
	if _, err := c.ParsePaymentWebhook(signWebhook(t, k, []byte(`{"eventId":"`+eid+`"}`), eid)); err != ErrInvalidWebhookBody {
		t.Fatalf("payment invalid body: %v", err)
	}
	if _, err := c.ParsePayoutWebhook(signWebhook(t, k, []byte(`{"eventId":"`+eid+`"}`), eid)); err != ErrInvalidWebhookBody {
		t.Fatalf("payout invalid body: %v", err)
	}
}

func TestParsePaymentWebhookPayer(t *testing.T) {
	k := newTestKeys(t)
	c := k.client(t, "https://api.example.com")
	eid := newEventID()
	for _, tc := range []struct {
		name      string
		payerJSON string
		want      *PaymentPayer
	}{
		{"absent", "", nil},
		{"null", `,"payer":null`, nil},
		{"empty", `,"payer":{}`, &PaymentPayer{}},
		{"complete", `,"payer":{"name":"Maria Silva","documentNumber":"01234567890"}`, &PaymentPayer{Name: "Maria Silva", DocumentNumber: "01234567890"}},
		{"name only", `,"payer":{"name":"Maria Silva"}`, &PaymentPayer{Name: "Maria Silva"}},
		{"accented name", `,"payer":{"name":"João da Silva","documentNumber":"01234567890"}`, &PaymentPayer{Name: "João da Silva", DocumentNumber: "01234567890"}},
		{"document only", `,"payer":{"documentNumber":"01234567890"}`, &PaymentPayer{DocumentNumber: "01234567890"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"eventId":"` + eid + `","orderType":"PAYMENT","orderNo":"P1","merchantOrderNo":"M1","status":"SUCCEEDED","amount":"100.50","paidAmount":"100.50","channelTradeNo":"E2E1"` + tc.payerJSON + `}`)
			got, err := c.ParsePaymentWebhook(signWebhook(t, k, body, eid))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Payer, tc.want) {
				t.Fatalf("payer=%+v, want %+v", got.Payer, tc.want)
			}
			if got.Amount != "100.50" || got.PaidAmount != "100.50" || got.ChannelTradeNo != "E2E1" {
				t.Fatalf("existing fields changed: %+v", got)
			}
			if tc.want != nil && tc.want.DocumentNumber != "" {
				r := signWebhook(t, k, body, eid)
				r.Body = io.NopCloser(strings.NewReader(strings.Replace(string(body), "01234567890", "11234567890", 1)))
				if _, err := c.ParsePaymentWebhook(r); err == nil {
					t.Fatal("tampered payer must fail verification")
				}
			}
		})
	}
}
