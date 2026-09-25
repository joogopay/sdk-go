package joogopay

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestPayoutRefundQueryAndSignedWebhook(t *testing.T) {
	k := newTestKeys(t)
	body := `{"orderNo":"PO1","merchantOrderNo":"M1","status":"REFUNDED","currency":"MXN","amount":"100.00","payoutMethod":"SPEI","action":{},"refundNo":"R1","refundAmount":"100.00","refundTime":1790208000000}`
	srv := verifyingServer(t, k, body, nil)
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())
	order, err := c.QueryPayoutByOrderNo(context.Background(), "PO1")
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != StatusRefunded || order.RefundNo != "R1" || order.RefundAmount != "100.00" || order.RefundTime != 1790208000000 {
		t.Fatalf("refund query: %+v", order)
	}
	eid := newEventID()
	payload, _ := json.Marshal(PayoutWebhook{EventID: eid, OrderType: WebhookOrderTypePayout, OrderNo: "PO1", MerchantOrderNo: "M1", Status: StatusRefunded, Currency: "MXN", Amount: "100.00", RefundNo: "R1", RefundAmount: "100.00", RefundTime: 1790208000000})
	hook, err := c.ParsePayoutWebhook(signWebhook(t, k, payload, eid))
	if err != nil {
		t.Fatal(err)
	}
	if hook.Status != order.Status || hook.RefundNo != order.RefundNo || hook.RefundAmount != order.RefundAmount || hook.RefundTime != order.RefundTime {
		t.Fatalf("refund webhook: %+v", hook)
	}
	var decoded map[string]any
	_ = json.Unmarshal(payload, &decoded)
	decoded["refundAmount"] = "200.00"
	tampered, _ := json.Marshal(decoded)
	req := signWebhook(t, k, payload, eid)
	req.Body = io.NopCloser(bytes.NewReader(tampered))
	if _, err := c.ParsePayoutWebhook(req); err == nil {
		t.Fatal("altered refund amount must fail verification")
	}
}
