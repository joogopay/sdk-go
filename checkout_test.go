package joogopay

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	merchantauth "github.com/joogopay/sdk-go/internal/merchantauth"
)

func TestGetPaymentCheckoutUnsigned(t *testing.T) {
	k := newTestKeys(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(merchantauth.HeaderSignature) != "" {
			t.Error("checkout must be unsigned")
		}
		_, _ = w.Write([]byte(`{"code":200,"msg":"OK","data":{"orderNo":"P1","status":"PENDING","amount":"100.50","currency":"BRL","params":{}}}`))
	}))
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())

	got, err := c.GetPaymentCheckout(context.Background(), "P1")
	if err != nil || got.OrderNo != "P1" || got.Amount != "100.50" {
		t.Fatalf("GetPaymentCheckout: %v %+v", err, got)
	}
	if _, err := c.GetPaymentCheckout(context.Background(), " "); err != ErrInvalidPathParam {
		t.Fatalf("blank orderNo: %v", err)
	}
	if _, err := c.GetPaymentCheckout(context.Background(), "a/b"); err != ErrInvalidPathParam {
		t.Fatalf("slash orderNo: %v", err)
	}
}

func TestSubmitPaymentTradeNoUnsigned(t *testing.T) {
	k := newTestKeys(t)
	var gotBody, gotPath, gotContentType string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(merchantauth.HeaderSignature) != "" {
			t.Error("submitTradeNo must be unsigned")
		}
		if r.Header.Get(merchantauth.HeaderContentEncryption) != "" {
			t.Error("submitTradeNo body must not be encrypted")
		}
		body, _ := io.ReadAll(r.Body)
		gotBody, gotPath, gotContentType = string(body), r.URL.Path, r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"code":200,"msg":"OK","data":{"status":1,"orderStatus":"PENDING"}}`))
	}))
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())

	got, err := c.SubmitPaymentTradeNo(context.Background(), "P1", "UTR123")
	if err != nil {
		t.Fatalf("SubmitPaymentTradeNo: %v", err)
	}
	if !got.Ok() || got.OrderStatus != "PENDING" {
		t.Fatalf("result = %+v", got)
	}
	if gotPath != "/api/v1/payment/submitTradeNo" {
		t.Errorf("path = %s", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %s", gotContentType)
	}
	if gotBody != `{"orderNo":"P1","tradeNo":"UTR123"}` {
		t.Errorf("body = %s", gotBody)
	}

	for _, tc := range [][2]string{{"", "UTR"}, {"P1", ""}, {" ", "UTR"}, {"P1", " "}} {
		if _, err := c.SubmitPaymentTradeNo(context.Background(), tc[0], tc[1]); err != ErrInvalidQueryParam {
			t.Errorf("SubmitPaymentTradeNo(%q,%q) = %v, want ErrInvalidQueryParam", tc[0], tc[1], err)
		}
	}
}

// TestSubmitPaymentTradeNoRefused pins the trap: the platform answers HTTP 200
// with envelope code 200 even when it refuses, so a nil error is not success.
func TestSubmitPaymentTradeNoRefused(t *testing.T) {
	k := newTestKeys(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":200,"msg":"OK","data":{"status":0,"message":"too many requests, please retry later"}}`))
	}))
	defer srv.Close()

	got, err := k.client(t, srv.URL, srv.Client()).SubmitPaymentTradeNo(context.Background(), "P1", "UTR123")
	if err != nil {
		t.Fatalf("refusal must not surface as a transport error: %v", err)
	}
	if got.Ok() {
		t.Fatal("status 0 must not report Ok")
	}
	if got.Message == "" {
		t.Fatal("refusal must carry a message")
	}
}

func TestAddPaymentExtraInfoUnsigned(t *testing.T) {
	k := newTestKeys(t)
	var gotBody, gotPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(merchantauth.HeaderSignature) != "" {
			t.Error("addExtraInfo must be unsigned")
		}
		body, _ := io.ReadAll(r.Body)
		gotBody, gotPath = string(body), r.URL.Path
		_, _ = w.Write([]byte(`{"code":200,"msg":"OK","data":{"status":1,"orderStatus":"PENDING","paymentUrl":"https://h5.example/p/1"}}`))
	}))
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())

	got, err := c.AddPaymentExtraInfo(context.Background(), "P1", "PK_JAZZCASH",
		map[string]string{"mobile": "03001234567"})
	if err != nil {
		t.Fatalf("AddPaymentExtraInfo: %v", err)
	}
	if !got.Ok() || got.PaymentUrl != "https://h5.example/p/1" {
		t.Fatalf("result = %+v", got)
	}
	if gotPath != "/api/v1/payment/addExtraInfo" {
		t.Errorf("path = %s", gotPath)
	}
	if gotBody != `{"orderNo":"P1","payMethod":"PK_JAZZCASH","extra":{"mobile":"03001234567"}}` {
		t.Errorf("body = %s", gotBody)
	}

	// payMethod / extra are optional and must be omitted rather than sent empty.
	if _, err := c.AddPaymentExtraInfo(context.Background(), "P1", "", nil); err != nil {
		t.Fatalf("optional fields: %v", err)
	}
	if gotBody != `{"orderNo":"P1"}` {
		t.Errorf("omitempty body = %s", gotBody)
	}

	if _, err := c.AddPaymentExtraInfo(context.Background(), " ", "", nil); err != ErrInvalidQueryParam {
		t.Fatalf("blank orderNo: %v", err)
	}
}
