package joogopay

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPaymentMethodSetExtra(t *testing.T) {
	var m PaymentMethod
	if err := m.SetExtra("", nil); err != ErrInvalidExtraField {
		t.Fatalf("empty field: %v", err)
	}
	if err := m.SetExtra("code", "x"); err != ErrInvalidExtraField {
		t.Fatalf("code field: %v", err)
	}
	if err := m.SetExtra("newMethod", map[string]any{"bankCode": "001"}); err != nil {
		t.Fatalf("SetExtra: %v", err)
	}
}

func TestPaymentMethodMarshalJSON(t *testing.T) {
	// typed field only
	m := PaymentMethod{Code: "PIX", Pix: &PaymentPixExtra{PayerName: "Maria"}}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"code":"PIX"`) || !strings.Contains(string(b), `"payerName":"Maria"`) {
		t.Fatalf("typed marshal: %s", b)
	}

	// extra merged into the object
	m2 := PaymentMethod{Code: "NEW_METHOD"}
	if err := m2.SetExtra("newMethod", map[string]any{"customerName": "X"}); err != nil {
		t.Fatal(err)
	}
	b2, err := json.Marshal(m2)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b2, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["newMethod"]; !ok {
		t.Fatalf("extra not merged: %s", b2)
	}
	if string(obj["code"]) != `"NEW_METHOD"` {
		t.Fatalf("code lost: %s", b2)
	}
}
