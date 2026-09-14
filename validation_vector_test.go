package joogopay

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Shared validation vectors: each case shallow-merges its override into a valid
// base request. A failure means the SDK disagrees with the protocol; fix the SDK,
// not the vectors.

type validationVector struct {
	Direction string                     `json:"direction"`
	Base      map[string]json.RawMessage `json:"base"`
	Cases     []struct {
		Name     string                     `json:"name"`
		Override map[string]json.RawMessage `json:"override"`
		Expect   string                     `json:"expect"`
		Reason   string                     `json:"reason"`
		Field    string                     `json:"field"`
		Untyped  bool                       `json:"untyped"`
	} `json:"cases"`
}

var validationReasons = map[string]error{
	"missing_required_field": ErrMissingRequiredField,
	"invalid_amount":         ErrInvalidAmount,
	"invalid_webhook_url":    ErrInvalidWebhookURL,
}

func TestValidationVectors(t *testing.T) {
	dir := filepath.Join("protocol", "testdata", "validation")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no validation vectors in %s: %v", dir, err)
	}
	sort.Strings(files)

	k := newTestKeys(t)
	srv := verifyingServer(t, k, `{"orderNo":"P1","merchantOrderNo":"M1","status":"PENDING","currency":"BRL","amount":"1.00","action":{}}`, nil)
	defer srv.Close()
	c := k.client(t, srv.URL, srv.Client())
	ctx := context.Background()

	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var v validationVector
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, tc := range v.Cases {
			t.Run(filepath.Base(file)+"/"+tc.Name, func(t *testing.T) {
				merged := map[string]json.RawMessage{}
				for k, val := range v.Base {
					merged[k] = val
				}
				for k, val := range tc.Override {
					merged[k] = val
				}
				body, _ := json.Marshal(merged)

				var callErr error
				switch v.Direction {
				case "payment":
					var req CreatePaymentReq
					if err := json.Unmarshal(body, &req); err != nil {
						if tc.Untyped && tc.Expect == "reject" {
							t.Skip("rejected by the type system; not expressible as a typed request")
						}
						t.Fatalf("decode request: %v", err)
					}
					_, callErr = c.CreatePayment(ctx, &req)
				case "payout":
					var req CreatePayoutReq
					if err := json.Unmarshal(body, &req); err != nil {
						if tc.Untyped && tc.Expect == "reject" {
							t.Skip("rejected by the type system; not expressible as a typed request")
						}
						t.Fatalf("decode request: %v", err)
					}
					_, callErr = c.CreatePayout(ctx, &req)
				default:
					t.Fatalf("unknown direction %q", v.Direction)
				}

				if tc.Expect == "accept" {
					if callErr != nil {
						t.Fatalf("should accept, got %v", callErr)
					}
					return
				}
				want, ok := validationReasons[tc.Reason]
				if !ok {
					t.Fatalf("unknown reason %q", tc.Reason)
				}
				if !errors.Is(callErr, want) {
					t.Fatalf("want %v, got %v", want, callErr)
				}
				if !errors.Is(callErr, ErrInvalidRequest) {
					t.Fatalf("%v should be under ErrInvalidRequest", callErr)
				}
				if tc.Field != "" && !strings.Contains(callErr.Error(), tc.Field) {
					t.Fatalf("error should name field %q: %v", tc.Field, callErr)
				}
			})
		}
	}
}
