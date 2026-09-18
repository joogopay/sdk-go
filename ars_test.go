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
	for _, field := range []string{"firstName", "lastName", "email", "phone", "address", "documentType", "documentNumber", "accountType", "accountNo"} {
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

func TestARSPayoutEmptyAddressWire(t *testing.T) {
	for _, rawExtra := range []bool{false, true} {
		for _, accountType := range []string{"CBU", "CVU"} {
			k := newTestKeys(t)
			cap := &capture{}
			srv := verifyingServer(t, k, `{}`, cap)
			t.Cleanup(srv.Close)
			method := arsPayoutMethod(accountType)
			method.BankTransfer.Address = ""
			if rawExtra {
				body, _ := json.Marshal(method.BankTransfer)
				var extra map[string]any
				if err := json.Unmarshal(body, &extra); err != nil {
					t.Fatal(err)
				}
				extra["address"] = ""
				if err := method.SetExtra("bankTransfer", extra); err != nil {
					t.Fatal(err)
				}
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
			if !present || address != "" {
				t.Fatalf("empty address was not transmitted: %s", cap.openedBody)
			}
		}
	}
}

func TestARSPayoutAddressRejectsMissingNullAndNonString(t *testing.T) {
	for _, tc := range []struct {
		name    string
		address any
		present bool
	}{
		{"missing", nil, false}, {"null", nil, true}, {"number", 1, true},
		{"boolean", false, true}, {"array", []any{}, true}, {"object", map[string]any{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			method := arsPayoutMethod("CBU")
			method.BankTransfer.Address = ""
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
			err := validatePayoutMethod(CurrencyARS, method)
			if err == nil || !strings.Contains(err.Error(), "extra.address") {
				t.Fatalf("invalid address accepted: %v", err)
			}
		})
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
		body, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), `"address"`) {
			t.Fatalf("%s unexpectedly sends an empty address: %s", currency, body)
		}
	}
}
