package joogopay

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestUSDWalletRequests(t *testing.T) {
	body, err := os.ReadFile("protocol/testdata/methods/001-usd-wallets.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Payment json.RawMessage   `json:"payment"`
		Payouts []json.RawMessage `json:"payouts"`
	}
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	for i, raw := range append([]json.RawMessage{fixture.Payment}, fixture.Payouts...) {
		var want map[string]any
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatal(err)
		}
		methodField := "payoutMethod"
		if i == 0 {
			methodField = "paymentMethod"
		}
		method := want[methodField].(map[string]any)
		code := method["code"].(string)
		t.Run(methodField+"/"+code, func(t *testing.T) {
			k := newTestKeys(t)
			cap := &capture{}
			srv := verifyingServer(t, k, `{}`, cap)
			defer srv.Close()
			c := k.client(t, srv.URL, srv.Client())
			if i == 0 {
				var req CreatePaymentReq
				if err := json.Unmarshal(raw, &req); err != nil {
					t.Fatal(err)
				}
				_, err = c.CreatePayment(context.Background(), &req)
			} else {
				var req CreatePayoutReq
				if err := json.Unmarshal(raw, &req); err != nil {
					t.Fatal(err)
				}
				_, err = c.CreatePayout(context.Background(), &req)
			}
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(cap.openedBody, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("typed USD request changed amount, account identifier or recipient fields")
			}
			branch := methodExtraFields[code]
			fields := method[branch].(map[string]any)
			for field := range fields {
				for _, empty := range []any{nil, "", "  "} {
					copyFields := map[string]any{}
					for k, v := range fields {
						copyFields[k] = v
					}
					copyFields[field] = empty
					cap.openedBody = nil
					if i == 0 {
						m := PaymentMethod{Code: code}
						if err := m.SetExtra(branch, copyFields); err != nil {
							t.Fatal(err)
						}
						_, err = c.CreatePayment(context.Background(), &CreatePaymentReq{MerchantOrderNo: "usd-missing-field", Currency: CurrencyUSD, Amount: "10.01", PaymentMethod: m, WebhookUrl: "https://merchant.example.com/webhook"})
					} else {
						m := PayoutMethod{Code: code}
						if err := m.SetExtra(branch, copyFields); err != nil {
							t.Fatal(err)
						}
						_, err = c.CreatePayout(context.Background(), &CreatePayoutReq{MerchantOrderNo: "usd-missing-field", Currency: CurrencyUSD, Amount: "10.01", PayoutMethod: m, WebhookUrl: "https://merchant.example.com/webhook"})
					}
					if !errors.Is(err, ErrMissingRequiredField) || cap.openedBody != nil {
						t.Fatalf("%s must fail before HTTP: %v", field, err)
					}
				}
			}
			// Formats remain the gateway's responsibility; do not add local regexes.
			for field := range fields {
				fields[field] = "format-is-checked-by-gateway"
			}
			methodJSON, _ := json.Marshal(method)
			rules := payoutMethodRules
			if i == 0 {
				rules = paymentMethodRules
			}
			if err := validateMethod(CurrencyUSD, methodJSON, code, rules); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUSDWalletMethodCodes(t *testing.T) {
	for _, code := range []string{MethodCodePayPal, MethodCodeChime} {
		raw, _ := json.Marshal(map[string]any{"code": code})
		if err := validateMethod(CurrencyUSD, raw, code, paymentMethodRules); !errors.Is(err, ErrMethodNotAvailable) {
			t.Fatalf("%s is payout-only: %v", code, err)
		}
	}
}
