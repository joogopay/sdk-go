package joogopay

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func arsPaymentMethod(code string) PaymentMethod {
	m := PaymentMethod{Code: code}
	switch code {
	case MethodCodeBankTransfer:
		m.BankTransfer = &PaymentBankTransferExtra{
			FirstName: "Ana", LastName: "Perez", Email: "ana@example.com",
			DocumentType: "DNI", DocumentNumber: "30123456",
		}
	case MethodCodeCVU:
		m.Cvu = &PaymentArsDocumentExtra{
			FirstName: "Ana", LastName: "Perez", Email: "ana@example.com", Phone: "1123456789",
			DocumentType: "DNI", DocumentNumber: "30123456",
		}
	case MethodCodeQRIS:
		m.Qris = &PaymentArsQrisExtra{
			FirstName: "Ana", LastName: "Perez", Email: "ana@example.com", Phone: "1123456789",
			DocumentType: "DNI", DocumentNumber: "30123456",
		}
	}
	return m
}

func arsPayoutMethod(accountType string) PayoutMethod {
	return PayoutMethod{Code: MethodCodeBankTransfer, BankTransfer: &PayoutBankTransferExtra{
		FirstName: "Ana", LastName: "Perez", Email: "ana@example.com", Phone: "1123456789",
		Address: "Av Example 123", DocumentType: "DNI", DocumentNumber: "30123456",
		AccountType: accountType, AccountNo: "0000003100012345678901",
	}}
}

func TestARSPaymentSignsSealsAndPreservesFields(t *testing.T) {
	for _, code := range []string{MethodCodeBankTransfer, MethodCodeCVU, MethodCodeQRIS} {
		t.Run(code, func(t *testing.T) {
			k := newTestKeys(t)
			cap := &capture{}
			srv := verifyingServer(t, k, `{"orderNo":"P1","currency":"ARS","amount":"1000.00","paymentMethod":"`+code+`","action":{"url":"https://checkout.example.com/ars"}}`, cap)
			defer srv.Close()
			c := k.client(t, srv.URL, srv.Client())
			req := &CreatePaymentReq{
				MerchantOrderNo: "ars-demo-001", Currency: CurrencyARS, Amount: "1000.00",
				PaymentMethod: arsPaymentMethod(code), WebhookUrl: "https://merchant.example.com/webhook",
			}
			order, err := c.CreatePayment(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if order.Action.Url != "https://checkout.example.com/ars" {
				t.Fatal("checkout URL was not preserved")
			}
			var got CreatePaymentReq
			if err := json.Unmarshal(cap.openedBody, &got); err != nil {
				t.Fatal(err)
			}
			if got.Amount != "1000.00" || got.Currency != CurrencyARS || got.PaymentMethod.Code != code {
				t.Fatal("ARS method, currency or decimal amount was not preserved")
			}
			wantMethod, _ := json.Marshal(req.PaymentMethod)
			gotMethod, _ := json.Marshal(got.PaymentMethod)
			if string(gotMethod) != string(wantMethod) {
				t.Fatal("ARS extra fields were not preserved through the signed, sealed request")
			}
			if code == MethodCodeBankTransfer && strings.Contains(string(gotMethod), `"phone"`) {
				t.Fatal("bank transfer must not require or invent a phone")
			}
		})
	}
}

func TestARSPayoutSignsSealsAndPreservesAccount(t *testing.T) {
	for _, accountType := range []string{"CBU", "CVU"} {
		t.Run(accountType, func(t *testing.T) {
			k := newTestKeys(t)
			cap := &capture{}
			srv := verifyingServer(t, k, `{"orderNo":"PO1","currency":"ARS","amount":"1000.00","payoutMethod":"BANK_TRANSFER","action":{}}`, cap)
			defer srv.Close()
			c := k.client(t, srv.URL, srv.Client())
			req := &CreatePayoutReq{
				MerchantOrderNo: "ars-payout-demo-001", Currency: CurrencyARS, Amount: "1000.00",
				PayoutMethod: arsPayoutMethod(accountType), WebhookUrl: "https://merchant.example.com/webhook",
			}
			if _, err := c.CreatePayout(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			var got CreatePayoutReq
			if err := json.Unmarshal(cap.openedBody, &got); err != nil {
				t.Fatal(err)
			}
			if got.Amount != req.Amount || got.PayoutMethod.BankTransfer == nil || *got.PayoutMethod.BankTransfer != *req.PayoutMethod.BankTransfer {
				t.Fatal("ARS recipient, semantic account type, leading zeros or decimal amount was not preserved")
			}
		})
	}
}

func TestARSRequiredFieldsRejectBeforeRequest(t *testing.T) {
	k := newTestKeys(t)
	cap := &capture{}
	srv := verifyingServer(t, k, `{}`, cap)
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())
	// Every request carries a complete envelope: the top-level check runs first
	// and would otherwise mask the method field under test.
	for _, code := range []string{MethodCodeBankTransfer, MethodCodeCVU, MethodCodeQRIS} {
		fields := []string{"firstName", "lastName", "email", "documentType", "documentNumber"}
		if code != MethodCodeBankTransfer {
			fields = append(fields, "phone")
		}
		for _, field := range fields {
			t.Run(code+"/"+field, func(t *testing.T) {
				m := arsPaymentMethod(code)
				body, _ := json.Marshal(m)
				var obj map[string]any
				if err := json.Unmarshal(body, &obj); err != nil {
					t.Fatal(err)
				}
				branch := methodExtraFields[code]
				extra := obj[branch].(map[string]any)
				delete(extra, field)
				if err := m.SetExtra(branch, extra); err != nil {
					t.Fatal(err)
				}
				_, err := c.CreatePayment(context.Background(), &CreatePaymentReq{
					MerchantOrderNo: "ars-missing-" + field, Currency: CurrencyARS, Amount: "1000.00",
					PaymentMethod: m, WebhookUrl: "https://merchant.example.com/webhook",
				})
				if !errors.Is(err, ErrMissingRequiredField) || !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "extra."+field) {
					t.Fatalf("missing %s: got %v", field, err)
				}
			})
		}
	}
	for _, field := range []string{"firstName", "lastName", "email", "phone", "documentType", "documentNumber", "accountType", "accountNo"} {
		t.Run("payout/"+field, func(t *testing.T) {
			m := arsPayoutMethod("CBU")
			body, _ := json.Marshal(m.BankTransfer)
			var extra map[string]any
			if err := json.Unmarshal(body, &extra); err != nil {
				t.Fatal(err)
			}
			delete(extra, field)
			if err := m.SetExtra("bankTransfer", extra); err != nil {
				t.Fatal(err)
			}
			_, err := c.CreatePayout(context.Background(), &CreatePayoutReq{
				MerchantOrderNo: "ars-missing-" + field, Currency: CurrencyARS, Amount: "1000.00",
				PayoutMethod: m, WebhookUrl: "https://merchant.example.com/webhook",
			})
			if !errors.Is(err, ErrMissingRequiredField) || !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "extra."+field) {
				t.Fatalf("missing %s: got %v", field, err)
			}
		})
	}
	if cap.openedBody != nil {
		t.Fatal("an invalid ARS request reached the server")
	}
}

func TestARSDocumentPhoneIsOptionalOutsideCVU(t *testing.T) {
	extra := &PaymentArsDocumentExtra{
		FirstName: "Ana", LastName: "Perez", Email: "ana@example.com",
		DocumentType: "DNI", DocumentNumber: "30123456",
	}
	body, err := json.Marshal(extra)
	if err != nil || strings.Contains(string(body), `"phone"`) {
		t.Fatal("empty optional phone must be omitted")
	}
	for _, m := range []PaymentMethod{
		{Code: MethodCodePagoFacil, PagoFacil: extra},
		{Code: MethodCodeRapipago, Rapipago: extra},
	} {
		if err := validatePaymentMethod(CurrencyARS, m); err != nil {
			t.Fatalf("existing method unexpectedly requires phone: %v", err)
		}
	}
}

func TestARSPayoutOptionalAddressWire(t *testing.T) {
	for _, accountType := range []string{"CBU", "CVU"} {
		for _, tc := range []struct {
			name    string
			raw     bool
			present bool
			address any
		}{
			{name: "typed-empty"},
			{name: "typed-string", present: true, address: " Av Example 123 "},
			{name: "missing", raw: true},
			{name: "null", raw: true, present: true},
			{name: "empty", raw: true, present: true, address: ""},
			{name: "string", raw: true, present: true, address: " Av Example 123 "},
		} {
			t.Run(accountType+"/"+tc.name, func(t *testing.T) {
				k := newTestKeys(t)
				cap := &capture{}
				srv := verifyingServer(t, k, `{}`, cap)
				t.Cleanup(srv.Close)
				method := arsPayoutMethod(accountType)
				method.BankTransfer.Address = ""
				if tc.raw {
					body, _ := json.Marshal(method.BankTransfer)
					var extra map[string]any
					if err := json.Unmarshal(body, &extra); err != nil {
						t.Fatal(err)
					}
					if tc.present {
						extra["address"] = tc.address
					}
					if err := method.SetExtra("bankTransfer", extra); err != nil {
						t.Fatal(err)
					}
				} else if tc.present {
					method.BankTransfer.Address = tc.address.(string)
				}
				req := &CreatePayoutReq{MerchantOrderNo: "ars-address-001", Currency: CurrencyARS, Amount: "1.00", WebhookUrl: "https://merchant.example.com/webhook", PayoutMethod: method}
				if _, err := k.client(t, srv.URL, srv.Client()).CreatePayout(context.Background(), req); err != nil {
					t.Fatal(err)
				}
				var opened struct {
					PayoutMethod struct {
						BankTransfer map[string]any `json:"bankTransfer"`
					} `json:"payoutMethod"`
				}
				if err := json.Unmarshal(cap.openedBody, &opened); err != nil {
					t.Fatal(err)
				}
				address, present := opened.PayoutMethod.BankTransfer["address"]
				if present != tc.present || address != tc.address {
					t.Fatalf("address presence or value changed: %s", cap.openedBody)
				}
			})
		}
	}
}

func TestARSPayoutAddressRejectsNonString(t *testing.T) {
	for _, accountType := range []string{"CBU", "CVU"} {
		for _, tc := range []struct {
			name    string
			address any
		}{
			{"number", 1}, {"boolean", false}, {"array", []any{}}, {"object", map[string]any{}},
		} {
			t.Run(accountType+"/"+tc.name, func(t *testing.T) {
				method := arsPayoutMethod(accountType)
				body, _ := json.Marshal(method.BankTransfer)
				var extra map[string]any
				if err := json.Unmarshal(body, &extra); err != nil {
					t.Fatal(err)
				}
				extra["address"] = tc.address
				if err := method.SetExtra("bankTransfer", extra); err != nil {
					t.Fatal(err)
				}
				k := newTestKeys(t)
				cap := &capture{}
				srv := verifyingServer(t, k, `{}`, cap)
				t.Cleanup(srv.Close)
				_, err := k.client(t, srv.URL, srv.Client()).CreatePayout(context.Background(), &CreatePayoutReq{
					MerchantOrderNo: "ars-invalid-address", Currency: CurrencyARS, Amount: "1000.00",
					PayoutMethod: method, WebhookUrl: "https://merchant.example.com/webhook",
				})
				if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "extra.address") {
					t.Fatalf("invalid address accepted: %v", err)
				}
				if cap.openedBody != nil {
					t.Fatal("an invalid address reached the server")
				}
			})
		}
	}
	for _, field := range []string{"documentType", "documentNumber"} {
		method := arsPayoutMethod("CBU")
		if field == "documentType" {
			method.BankTransfer.DocumentType = ""
		} else {
			method.BankTransfer.DocumentNumber = ""
		}
		if err := validatePayoutMethod(CurrencyARS, method); err == nil {
			t.Fatalf("empty %s accepted", field)
		}
	}
}

func TestPayoutEmptyAddressOmittedOutsideARS(t *testing.T) {
	for _, currency := range []string{"MXN", "PEN", "CLP"} {
		req := CreatePayoutReq{Currency: currency, PayoutMethod: PayoutMethod{Code: MethodCodeBankTransfer, BankTransfer: &PayoutBankTransferExtra{AccountNo: "123"}}}
		body, err := req.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), `"address"`) {
			t.Fatalf("%s unexpectedly sends an empty address: %s", currency, body)
		}
	}
}

func TestARSAddressTypeCheckDoesNotAffectOtherRequests(t *testing.T) {
	payment := arsPaymentMethod(MethodCodeBankTransfer)
	body, err := json.Marshal(payment.BankTransfer)
	if err != nil {
		t.Fatal(err)
	}
	var payer map[string]any
	if err := json.Unmarshal(body, &payer); err != nil {
		t.Fatal(err)
	}
	payer["address"] = 1
	if err := payment.SetExtra("bankTransfer", payer); err != nil {
		t.Fatal(err)
	}
	if err := validatePaymentMethod(CurrencyARS, payment); err != nil {
		t.Fatalf("payout-only address rule affected a payment: %v", err)
	}
	payout := PayoutMethod{Code: MethodCodeBankTransfer}
	if err := payout.SetExtra("bankTransfer", map[string]any{
		"accountName": "Ana Perez", "accountNo": "012345678901234567", "accountType": "CLABE",
		"bankCode": "40012", "bankName": "Example Bank", "address": 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := validatePayoutMethod(CurrencyMXN, payout); err != nil {
		t.Fatalf("ARS address rule affected another currency: %v", err)
	}
}

func TestARSPayoutSetExtraReplacesTypedRecipient(t *testing.T) {
	for _, accountType := range []string{"CBU", "CVU"} {
		for _, tc := range []struct {
			name, address            string
			present, missingDocument bool
		}{
			{name: "missing-address"},
			{name: "null-address", present: true, address: "null"},
			{name: "empty-address", present: true, address: `""`},
			{name: "replacement-address", present: true, address: `"Av. Córdoba 123, 2º \"B\"\nCABA"`},
			{name: "missing-document", missingDocument: true},
		} {
			t.Run(accountType+"/"+tc.name, func(t *testing.T) {
				method := arsPayoutMethod(accountType)
				typedBefore := *method.BankTransfer
				body, err := json.Marshal(method.BankTransfer)
				if err != nil {
					t.Fatal(err)
				}
				var extra map[string]any
				if err := json.Unmarshal(body, &extra); err != nil {
					t.Fatal(err)
				}
				delete(extra, "address")
				if tc.present {
					var address any
					if err := json.Unmarshal([]byte(tc.address), &address); err != nil {
						t.Fatal(err)
					}
					extra["address"] = address
				}
				if tc.missingDocument {
					delete(extra, "documentNumber")
				}
				wantExtra, err := json.Marshal(extra)
				if err != nil {
					t.Fatal(err)
				}
				if err := method.SetExtra("bankTransfer", extra); err != nil {
					t.Fatal(err)
				}
				k, cap := newTestKeys(t), &capture{}
				srv := verifyingServer(t, k, `{}`, cap)
				t.Cleanup(srv.Close)
				_, err = k.client(t, srv.URL, srv.Client()).CreatePayout(context.Background(), &CreatePayoutReq{
					MerchantOrderNo: "ars-replaced-recipient", Currency: CurrencyARS, Amount: "1000.00",
					PayoutMethod: method, WebhookUrl: "https://merchant.example.com/webhook",
				})
				if tc.missingDocument {
					if !errors.Is(err, ErrMissingRequiredField) || !strings.Contains(err.Error(), "extra.documentNumber") {
						t.Fatalf("typed recipient must not supply the removed document: %v", err)
					}
					if cap.openedBody != nil {
						t.Fatal("incomplete replacement reached the server")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					var opened struct {
						PayoutMethod struct {
							BankTransfer map[string]any `json:"bankTransfer"`
						} `json:"payoutMethod"`
					}
					if err := json.Unmarshal(cap.openedBody, &opened); err != nil {
						t.Fatal(err)
					}
					gotExtra, err := json.Marshal(opened.PayoutMethod.BankTransfer)
					if err != nil {
						t.Fatal(err)
					}
					if string(gotExtra) != string(wantExtra) {
						t.Fatalf("replacement changed on the wire: got %s, want %s", gotExtra, wantExtra)
					}
				}
				after, err := json.Marshal(extra)
				if err != nil {
					t.Fatal(err)
				}
				if *method.BankTransfer != typedBefore || string(after) != string(wantExtra) {
					t.Fatal("validation or serialization mutated the caller's recipient")
				}
			})
		}
	}
}
