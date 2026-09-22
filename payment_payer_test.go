package joogopay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaymentQueryPayer(t *testing.T) {
	for _, tt := range []struct {
		name, fields string
		want         *PaymentPayer
	}{
		{"present", `,"payer":{"name":"Maria Silva","documentNumber":"01234567890"}`, &PaymentPayer{Name: "Maria Silva", DocumentNumber: "01234567890"}},
		{"absent", "", nil},
		{"null", `,"payer":null`, nil},
		{"partial", `,"payer":{"documentNumber":"01234567890"}`, &PaymentPayer{DocumentNumber: "01234567890"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			keys := newTestKeys(t)
			server := verifyingServer(t, keys, `{"orderNo":"P1","status":"SUCCEEDED"`+tt.fields+`}`, nil)
			defer server.Close()
			client := keys.client(t, server.URL, server.Client())
			for _, query := range []func(context.Context, string) (*PaymentOrder, error){client.QueryPaymentByOrderNo, client.QueryPaymentByMerchantOrderNo} {
				order, err := query(context.Background(), "P1")
				if err != nil {
					t.Fatal(err)
				}
				if tt.want == nil {
					if order.Payer != nil {
						t.Fatalf("unexpected payer: %+v", order.Payer)
					}
				} else if order.Payer == nil || *order.Payer != *tt.want {
					t.Fatalf("payer = %+v, want %+v", order.Payer, tt.want)
				}
			}
		})
	}
}

func TestPaymentQueryRejectsUntrustedTLSCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request reached an untrusted HTTPS endpoint")
	}))
	defer server.Close()
	client := newTestKeys(t).client(t, server.URL, &http.Client{})
	order, err := client.QueryPaymentByOrderNo(context.Background(), "P1")
	if order != nil || !errors.Is(err, ErrTransport) {
		t.Fatalf("order=%+v err=%v", order, err)
	}
}
