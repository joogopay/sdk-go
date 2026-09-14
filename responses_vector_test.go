package joogopay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The response vectors are shared by every SDK; these tests keep the Go types
// and the amounts-as-decimal-strings rule aligned with them.

func readVector(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("protocol", "testdata", "responses", name))
	if err != nil {
		t.Fatalf("read vector %s: %v", name, err)
	}
	var fixture map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse vector %s: %v", name, err)
	}
	return fixture
}

func vectorData(t *testing.T, fixture map[string]json.RawMessage) json.RawMessage {
	t.Helper()
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixture["body"], &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	return body.Data
}

func TestResponseVectorSuccessPaymentOrder(t *testing.T) {
	fixture := readVector(t, "001-success-payment-order.json")

	var order PaymentOrder
	if err := json.Unmarshal(vectorData(t, fixture), &order); err != nil {
		t.Fatalf("vector does not decode into PaymentOrder: %v", err)
	}
	if order.OrderNo != "ORD202605190001" {
		t.Errorf("OrderNo = %q, want ORD202605190001", order.OrderNo)
	}
	if order.Status != StatusSucceeded {
		t.Errorf("Status = %q, want %q", order.Status, StatusSucceeded)
	}
	if order.Amount != "100.00" || order.PaidAmount != "100.00" {
		t.Errorf("amount = %q / paidAmount = %q, want decimal string 100.00",
			order.Amount, order.PaidAmount)
	}
	if order.Action.QrCode != "00020-qr" {
		t.Errorf("Action.QrCode = %q, want 00020-qr", order.Action.QrCode)
	}
}

func TestResponseVectorAmountsAreDecimalStrings(t *testing.T) {
	moneyFields := []string{
		"amount", "paidAmount", "minAmount", "maxAmount", "usdRate",
		"balance", "lockBalance", "paymentBalance", "paymentLockBalance",
		"payoutBalance", "payoutLockBalance",
	}
	entries, err := os.ReadDir(filepath.Join("protocol", "testdata", "responses"))
	if err != nil {
		t.Fatalf("read responses dir: %v", err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		fixture := readVector(t, entry.Name())
		body, ok := fixture["body"]
		if !ok {
			continue
		}
		var decoded struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(body, &decoded); err != nil {
			continue
		}
		for _, field := range moneyFields {
			value, ok := decoded.Data[field]
			if !ok {
				continue
			}
			if _, isString := value.(string); !isString {
				t.Errorf("%s: %s = %v (%T), want decimal string",
					entry.Name(), field, value, value)
			}
		}
	}
}

func TestResponseVectorAPIError(t *testing.T) {
	fixture := readVector(t, "002-api-error-order-not-found.json")

	var envelopeValue envelope
	if err := json.Unmarshal(fixture["body"], &envelopeValue); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	var want struct {
		Code    int    `json:"Code"`
		Msg     string `json:"Msg"`
		Message string `json:"Message"`
		TraceID string `json:"TraceID"`
	}
	if err := json.Unmarshal(fixture["expectedFields"], &want); err != nil {
		t.Fatalf("parse expectedFields: %v", err)
	}
	if envelopeValue.Code != want.Code || envelopeValue.Msg != want.Msg ||
		envelopeValue.TraceID != want.TraceID || envelopeValue.dataMessage() != want.Message {
		t.Errorf("envelope = {%d %s %s %s}, want {%d %s %s %s}",
			envelopeValue.Code, envelopeValue.Msg, envelopeValue.TraceID, envelopeValue.dataMessage(),
			want.Code, want.Msg, want.TraceID, want.Message)
	}
}

func TestResponseVectorHTTP200EnvelopeNotOK(t *testing.T) {
	fixture := readVector(t, "003-http-200-envcode-not-ok.json")

	var httpStatus int
	if err := json.Unmarshal(fixture["httpStatus"], &httpStatus); err != nil {
		t.Fatalf("parse httpStatus: %v", err)
	}
	var envelopeValue envelope
	if err := json.Unmarshal(fixture["body"], &envelopeValue); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	if httpStatus != 200 {
		t.Fatalf("httpStatus = %d, want 200", httpStatus)
	}
	if envelopeValue.Code == 200 {
		t.Fatal("vector envelope code should not be 200")
	}
}
