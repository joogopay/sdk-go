package joogopay

import (
	"errors"
	"fmt"
	"testing"
)

// The rules table is generated; these tests guard how it is applied. Half of the
// cases assert acceptance: a valid request wrongly rejected can only be fixed by
// an SDK release.

func TestValidatePaymentMethodStructure(t *testing.T) {
	cases := []struct {
		name     string
		currency string
		method   PaymentMethod
		want     error
	}{
		{"missing code", "BRL", PaymentMethod{}, ErrMissingMethodCode},
		{"two extras conflict", "PKR", PaymentMethod{
			Code:        "PK_JAZZCASH",
			PkJazzcash:  &PaymentPkWalletExtra{Mobile: "03001234567"},
			PkEasypaisa: &PaymentPkWalletExtra{Mobile: "03001234567"},
		}, ErrConflictingMethodExtra},
		{"extra does not match code", "PKR", PaymentMethod{
			Code:        "PK_JAZZCASH",
			PkEasypaisa: &PaymentPkWalletExtra{Mobile: "03001234567"},
		}, ErrMethodExtraMismatch},
		{"method not available for currency", "PKR", PaymentMethod{
			Code: "PH_GCASH", PhGcash: &PaymentAccountContactExtra{},
		}, ErrMethodNotAvailable},
		{"IDR pay-in missing bankCode", "IDR", PaymentMethod{
			Code: "ID_VA",
			IdVa: &PaymentBankAccountContactExtra{
				AccountName: "Budi", Email: "b@example.com", Mobile: "081234567890",
			},
		}, ErrMissingRequiredField},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePaymentMethod(tc.currency, tc.method)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

// TestValidatePaymentMethodAllowsValid covers requests that pass validation.
func TestValidatePaymentMethodAllowsValid(t *testing.T) {
	cases := []struct {
		name     string
		currency string
		method   PaymentMethod
	}{
		// PKR / PHP pay-ins have no gateway-required extras; omitting extra entirely is valid
		{"PKR without extra", "PKR", PaymentMethod{Code: "PK_JAZZCASH"}},
		{"PKR partial extra", "PKR", PaymentMethod{
			Code: "PK_JAZZCASH", PkJazzcash: &PaymentPkWalletExtra{Mobile: "03001234567"},
		}},
		{"PHP without extra", "PHP", PaymentMethod{Code: "PH_GCASH"}},
		{"IDR all fields", "IDR", PaymentMethod{
			Code: "ID_VA",
			IdVa: &PaymentBankAccountContactExtra{
				AccountName: "Budi", Email: "b@example.com", Mobile: "081234567890", BankCode: "BCA",
			},
		}},
		// a currency missing from the table passes through to the gateway; the SDK table may lag behind
		{"unknown currency passes", "XYZ", PaymentMethod{Code: "WHATEVER"}},
		// a currency without a method allowlist never rejects on code
		{"BRL has no allowlist", "BRL", PaymentMethod{Code: "PIX"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validatePaymentMethod(tc.currency, tc.method); err != nil {
				t.Fatalf("valid request rejected: %v", err)
			}
		})
	}
}

// TestValidatePayoutConditionalRequired guards per-method required fields within
// one currency: flattening the rules into an unconditional table would wrongly
// reject IN_UPI payouts.
func TestValidatePayoutConditionalRequired(t *testing.T) {
	upi := PayoutMethod{Code: "IN_UPI", InUpi: &PayoutInUpiExtra{
		Account: "mary@upi", Name: "Mary", Email: "m@example.com", Mobile: "9871476369",
	}}
	if err := validatePayoutMethod("INR", upi); err != nil {
		t.Fatalf("IN_UPI needs no ifsc/account, got %v", err)
	}

	ifscMissing := PayoutMethod{Code: "IN_IFSC", InIfsc: &PayoutInIfscExtra{
		Name: "Mary", Email: "m@example.com", Mobile: "9871476369",
	}}
	if err := validatePayoutMethod("INR", ifscMissing); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("IN_IFSC missing ifsc/account: want ErrMissingRequiredField, got %v", err)
	}

	ifscOk := PayoutMethod{Code: "IN_IFSC", InIfsc: &PayoutInIfscExtra{
		Account: "123456789", Ifsc: "HDFC0001234",
		Name: "Mary", Email: "m@example.com", Mobile: "9871476369",
	}}
	if err := validatePayoutMethod("INR", ifscOk); err != nil {
		t.Fatalf("complete IN_IFSC rejected: %v", err)
	}

	// PKR payouts: only PK_BANK requires bankCode
	wallet := PayoutMethod{Code: "PK_JAZZCASH", PkJazzcash: &PayoutPkDfExtra{
		AccountNo: "03001234567", Cnic: "1234512345671", Mobile: "03001234567",
	}}
	if err := validatePayoutMethod("PKR", wallet); err != nil {
		t.Fatalf("PK wallet payout needs no bankCode, got %v", err)
	}
	bank := PayoutMethod{Code: "PK_BANK", PkBank: &PayoutPkDfExtra{
		AccountNo: "123456", Cnic: "1234512345671", Mobile: "03001234567",
	}}
	if err := validatePayoutMethod("PKR", bank); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("PK_BANK missing bankCode: want ErrMissingRequiredField, got %v", err)
	}
}

// TestValidateSeesSetExtra checks that validation, running on the serialized
// shape, sees SetExtra payloads too.
func TestValidateSeesSetExtra(t *testing.T) {
	m := PaymentMethod{Code: "PK_JAZZCASH"}
	if err := m.SetExtra("phGcash", map[string]any{"mobile": "09171234567"}); err != nil {
		t.Fatalf("SetExtra: %v", err)
	}
	if err := validatePaymentMethod("PKR", m); !errors.Is(err, ErrMethodExtraMismatch) {
		t.Fatalf("SetExtra under the wrong field: want ErrMethodExtraMismatch, got %v", err)
	}
}

// TestErrInvalidRequestCoversPreflight pins the pre-flight sentinels under
// ErrInvalidRequest; TestSendPathErrorsAreClassified covers the send paths
// themselves. Integrators rely on one errors.Is to decide whether a failure is
// safe to mark failed; an unwrapped error is classified as an unknown outcome
// and strands a payout in pending, and neither the compiler nor the other tests
// would notice.
func TestErrInvalidRequestCoversPreflight(t *testing.T) {
	preflight := []error{
		ErrNilRequest,
		ErrInvalidPathParam,
		ErrInvalidQueryParam,
		ErrInvalidExtraField,
		ErrInvalidIdempotencyKey,
		ErrMissingMethodCode,
		ErrConflictingMethodExtra,
		ErrMethodExtraMismatch,
		ErrMethodNotAvailable,
		ErrMissingRequiredField,
		ErrSignedPostQueryNotAllowed,
		ErrSignedGetBodyNotAllowed,
		ErrSignedMethodNotSupported,
	}
	for _, err := range preflight {
		if !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%v is not wrapped under ErrInvalidRequest", err)
		}
		// callers wrap once more for context; the sentinel has to survive that
		if wrapped := fmt.Errorf("createPayout: %w", err); !errors.Is(wrapped, ErrInvalidRequest) {
			t.Errorf("wrapped %v no longer matches ErrInvalidRequest", err)
		}
		// each sentinel keeps its own identity; callers may match a specific one
		if !errors.Is(err, err) {
			t.Errorf("%v no longer matches itself", err)
		}
	}
}

// TestErrInvalidRequestExcludesUncertain pins the reverse: errors with an
// unknown outcome stay outside ErrInvalidRequest, otherwise a payout the
// upstream may already have paid gets marked failed.
func TestErrInvalidRequestExcludesUncertain(t *testing.T) {
	for _, err := range []error{
		ErrResponseTooLarge,                    // request already sent
		&APIError{HTTPStatus: 400, Code: 1001}, // explicit upstream answer, handled via AsAPIError
		&ResponseError{HTTPStatus: 502},        // non-envelope response, undecidable
		errors.New("dial tcp: i/o timeout"),    // network error
		ErrMissingBaseURL,                      // constructor config error, a separate class
		ErrInvalidWebhookBody,                  // webhook side, not on the request path
	} {
		if errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%v wrongly matches ErrInvalidRequest", err)
		}
	}
}

func validPaymentReq() *CreatePaymentReq {
	return &CreatePaymentReq{
		MerchantOrderNo: "M1", Currency: "BRL", Amount: "100.50",
		PaymentMethod: PaymentMethod{Code: "PIX"}, WebhookUrl: "https://m.example.com/hook",
	}
}

func validPayoutReq() *CreatePayoutReq {
	return &CreatePayoutReq{
		MerchantOrderNo: "M2", Currency: "BRL", Amount: "10.00",
		PayoutMethod: PayoutMethod{Code: "PIX", Pix: &PayoutPixExtra{KeyType: "CPF", Key: "12345678901"}},
		WebhookUrl:   "https://m/h",
	}
}

func TestValidatePathOrderNo(t *testing.T) {
	for _, bad := range []string{"", "  "} {
		if _, err := validatePathOrderNo(bad); !errors.Is(err, ErrInvalidPathParam) {
			t.Fatalf("%q: got %v", bad, err)
		}
	}
	got, err := validatePathOrderNo("  P202608270001 ")
	if err != nil || got != "P202608270001" {
		t.Fatalf("got %q, %v", got, err)
	}
}
