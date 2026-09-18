package joogopay

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Format checks (phone digits, e-mail, IFSC) stay with the gateway: those rules
// evolve per currency and channel, a copy here would drift, and a merchant could
// only get a fix through an SDK release while the gateway's 400 is always current.

// validateAmount requires value to match amountPattern (the gateway's order
// amount regex plus the DECIMAL(18,2) ceiling) and to be greater than zero; the
// pattern admits only digits and a dot, so > 0 reduces to "has a non-zero digit".
func validateAmount(field, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s", ErrMissingRequiredField, field)
	}
	if !amountPattern.MatchString(value) {
		return fmt.Errorf("%w: %s=%q", ErrInvalidAmount, field, value)
	}
	if !strings.ContainsAny(value, "123456789") {
		return fmt.Errorf("%w: %s must be greater than 0", ErrInvalidAmount, field)
	}
	return nil
}

// validateWebhookURL applies the same rule as the gateway: required, and an absolute https:// URL.
func validateWebhookURL(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s", ErrMissingRequiredField, field)
	}
	if !strings.HasPrefix(strings.ToLower(value), webhookURLPrefix) {
		return fmt.Errorf("%w: %s must be an absolute %s URL", ErrInvalidWebhookURL, field, strings.TrimSuffix(webhookURLPrefix, "://"))
	}
	return nil
}

// validateCreateCommon checks the four fields the gateway marks required on
// create, so an empty value fails before signing and sealing instead of coming
// back as INVALID_FIELD.
func validateCreateCommon(merchantOrderNo, currency, amount, webhookURL string) error {
	values := map[string]string{"merchantOrderNo": merchantOrderNo, "currency": currency}
	for _, field := range createRequiredTextFields {
		if strings.TrimSpace(values[field]) == "" {
			return fmt.Errorf("%w: %s", ErrMissingRequiredField, field)
		}
	}
	if err := validateAmount("amount", amount); err != nil {
		return err
	}
	return validateWebhookURL("webhookUrl", webhookURL)
}

// validatePathOrderNo trims and requires a non-empty order number. Special
// characters pass through: the caller url.PathEscapes them before signing, so
// "ORD/001" signs as "ORD%2F001" (pinned by every SDK's tests).
func validatePathOrderNo(orderNo string) (string, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return "", ErrInvalidPathParam
	}
	return orderNo, nil
}

// emptyExtraValue treats nil and blank strings as missing; any other value,
// false and 0 included, counts as present.
func emptyExtraValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	default:
		return false
	}
}

// validateMethod inspects the serialized method exactly as it goes on the wire,
// so payloads injected via SetExtra are checked like typed fields.
func validateMethod(currency string, raw []byte, code string, rules map[string]methodRule) error {
	if code = strings.TrimSpace(code); code == "" {
		return ErrMissingMethodCode
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("%w: encode method: %w", ErrInvalidRequest, err)
	}
	present := make([]string, 0, len(obj))
	for k := range obj {
		if k != "code" {
			present = append(present, k)
		}
	}
	sort.Strings(present)

	if len(present) > 1 {
		return fmt.Errorf("%w: %s", ErrConflictingMethodExtra, strings.Join(present, ", "))
	}
	if want := methodExtraFields[code]; len(present) == 1 && want != "" && present[0] != want {
		return fmt.Errorf("%w: %s expects %q, got %q", ErrMethodExtraMismatch, code, want, present[0])
	}

	rule, ok := rules[strings.ToUpper(strings.TrimSpace(currency))]
	if !ok {
		// No rules for this currency: let the gateway decide, the SDK table may lag behind it.
		return nil
	}
	if len(rule.codes) > 0 {
		found := false
		for _, c := range rule.codes {
			if c == code {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: %s for %s", ErrMethodNotAvailable, code, currency)
		}
	}

	need := append(append([]string(nil), rule.required...), rule.byMethod[code]...)
	optionalStrings := rule.optionalNullableStringsByMethod[code]
	if len(need) == 0 && len(optionalStrings) == 0 {
		return nil
	}
	extra := map[string]any{}
	if len(present) == 1 {
		if err := json.Unmarshal(obj[present[0]], &extra); err != nil {
			return fmt.Errorf("%w: encode method extra: %w", ErrInvalidRequest, err)
		}
	}
	for _, f := range need {
		if slices.Contains(rule.allowEmpty, f) {
			if _, ok := extra[f].(string); !ok {
				return fmt.Errorf("%w: extra.%s must be a string for %s %s", ErrMissingRequiredField, f, currency, code)
			}
			continue
		}
		if v, ok := extra[f]; !ok || emptyExtraValue(v) {
			return fmt.Errorf("%w: extra.%s for %s %s", ErrMissingRequiredField, f, currency, code)
		}
	}
	for _, f := range optionalStrings {
		if value := extra[f]; value != nil {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%w: extra.%s must be a string or null for %s %s", ErrInvalidRequest, f, currency, code)
			}
		}
	}
	return nil
}

func validatePaymentMethod(currency string, m PaymentMethod) error {
	raw, err := m.MarshalJSON()
	if err != nil {
		return fmt.Errorf("%w: encode paymentMethod: %w", ErrInvalidRequest, err)
	}
	return validateMethod(currency, raw, m.Code, paymentMethodRules)
}

func validatePayoutMethod(currency string, m PayoutMethod) error {
	raw, err := m.MarshalJSON()
	if err != nil {
		return fmt.Errorf("%w: encode payoutMethod: %w", ErrInvalidRequest, err)
	}
	return validateMethod(currency, raw, m.Code, payoutMethodRules)
}
