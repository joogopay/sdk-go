// Package joogopay is the Go SDK for the merchant open API.
//
// Requests are signed with the merchant's Ed25519 key using RFC 9421 HTTP
// Message Signatures; POST bodies are sealed to the platform's X25519 public
// key (libsodium sealed box). Signing, digesting and sealing are handled
// transparently; callers deal with typed requests and responses.
//
//	c, err := joogopay.NewClient(joogopay.Config{
//	    BaseURL:                     "https://panama.joogopay.com",
//	    AccessKey:                   "mak_live_xxx",
//	    MerchantPrivateKeyBase64:    merchantPrivateKey,
//	    PlatformBodyKeyID:           "body_20260827_01",
//	    PlatformBodyPublicKeyBase64: platformBodyPublicKey,
//	    PlatformWebhookPublicKeys:   platformWebhookPublicKeys,
//	})
//	order, err := c.CreatePayment(ctx, &joogopay.CreatePaymentReq{...})
//
// Amounts, balances, fees and rates are decimal strings, never JSON numbers.
// Write APIs are not retried automatically; recover from timeouts by querying
// the order via merchantOrderNo or orderNo. A deliberate retry reuses the same
// merchantOrderNo, which is the only key the platform deduplicates on; the
// idempotency key is carried for tracing only. The wire protocol is documented
// under protocol/ and the shared conformance vectors under protocol/testdata.
package joogopay
