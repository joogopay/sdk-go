# JooGoPay SDK for Go

Go SDK for the merchant open API. **English** | [简体中文](./README-cn.md)

Requests are signed with the merchant's Ed25519 key (RFC 9421 HTTP Message
Signatures). POST bodies are sealed to the platform's X25519 public key
(libsodium sealed box). Signing, digesting and sealing are handled by the SDK.
`NewClient` rejects non-HTTPS BaseURLs. Synchronous responses remain plaintext
JSON and rely on HTTPS/TLS for confidentiality and integrity.

## 0. Installation

```
go get github.com/joogopay/sdk-go
```

Go 1.24 or newer. The only third-party dependency is `golang.org/x/crypto`.

## 1. Usage

```go
import joogopay "github.com/joogopay/sdk-go"

c, err := joogopay.NewClient(joogopay.Config{
    BaseURL:                     "https://panama.joogopay.com",     // production origin; no /api/v1
    AccessKey:                   "mak_live_xxx",
    MerchantPrivateKeyBase64:    merchantPrivateKey,            // merchant Ed25519 private key, base64
    PlatformBodyKeyID:           "body_20260827_01",
    PlatformBodyPublicKeyBase64: platformBodyPublicKey,         // platform X25519 public key, base64
    PlatformWebhookPublicKeys:   platformWebhookPublicKeys,     // keyID -> base64 Ed25519 public key
})
if err != nil { return err }

order, err := c.CreatePayment(ctx, &joogopay.CreatePaymentReq{
    MerchantOrderNo: "M20260101001",
    Currency:        joogopay.CurrencyBRL,
    Amount:          "100.00",                                  // decimal string, never a JSON number
    PaymentMethod: joogopay.PaymentMethod{
        Code: joogopay.MethodCodePIX,
        Pix:  &joogopay.PaymentPixExtra{PayerName: "Joao Silva"},
    },
    WebhookUrl: "https://merchant.example/webhook/payments",
})

// Error handling
var apiErr *joogopay.APIError
var respErr *joogopay.ResponseError
switch {
case errors.As(err, &apiErr):  // business error (envelope decoded)
    log.Printf("api error: %s %s (trace=%s)", apiErr.Msg, apiErr.Message, apiErr.TraceID)
case errors.As(err, &respErr): // infrastructure error (gateway/CDN returned a non-envelope body)
case err != nil:               // transport error
}
```

Keys are the merchant's responsibility: the merchant generates its own Ed25519
key pair and shares only the public key with the platform. The platform never
receives or stores the merchant private key.

### ARS payments and payouts

ARS payments use `BANK_TRANSFER` / `BankTransfer`, `CVU` / `Cvu`, or `QRIS` / `Qris`.
All three require `FirstName`, `LastName`, `Email`, `DocumentType` (DNI/CUIT), and
`DocumentNumber`. CVU and QRIS also require `Phone`: 10 digits, with a nonzero first
digit. BANK_TRANSFER does not require a phone. The identity and account details below
are fictional; replace them with the actual customer details.

```go
order, err := c.CreatePayment(ctx, &joogopay.CreatePaymentReq{
    MerchantOrderNo: "ars-cvu-demo-001", Currency: joogopay.CurrencyARS, Amount: "1000.00",
    PaymentMethod: joogopay.PaymentMethod{
        Code: joogopay.MethodCodeCVU,
        Cvu: &joogopay.PaymentArsDocumentExtra{
            FirstName: "Ana", LastName: "Perez", Email: "ana@example.com", Phone: "1123456789",
            DocumentType: "DNI", DocumentNumber: "30123456",
        },
    },
    WebhookUrl: "https://merchant.example.com/webhook",
})
```

Redirect the payer to the channel checkout at `order.Action.Url`. Select each method
explicitly; they do not switch automatically. Opening the checkout or returning to
your site does not confirm payment. Use order queries or verified platform webhooks.

ARS payouts use `BANK_TRANSFER`. Set `AccountType` to the string `"CBU"` or `"CVU"`,
and keep `AccountNo` as a digit string to preserve leading zeros. The same phone
format applies, and all nine recipient fields are required:

```go
order, err := c.CreatePayout(ctx, &joogopay.CreatePayoutReq{
    MerchantOrderNo: "ars-payout-demo-001", Currency: joogopay.CurrencyARS, Amount: "1000.00",
    PayoutMethod: joogopay.PayoutMethod{
        Code: joogopay.MethodCodeBankTransfer,
        BankTransfer: &joogopay.PayoutBankTransferExtra{
            FirstName: "Ana", LastName: "Perez", Email: "ana@example.com", Phone: "1123456789",
            Address: "Av Example 123", DocumentType: "DNI", DocumentNumber: "30123456",
            AccountNo: "0000003100012345678901", AccountType: "CBU", // "CVU" is also supported
        },
    },
    WebhookUrl: "https://merchant.example.com/webhook",
})
```

The SDK checks required fields before sending. The gateway validates phone format
and account type values. Other currencies retain their own account type rules.

### Two kinds of failure, opposite handling

| Error | Meaning | Action |
| --- | --- | --- |
| `errors.Is(err, ErrInvalidRequest)` | Rejected **before it was sent** (local validation, a bad parameter, or a request the SDK could not encode or sign) | Safe to mark failed; fix the request and retry under the same `merchantOrderNo` |
| `errors.Is(err, ErrTransport)` | Handed to the transport, no usable response (connection failure, timeout, interrupted read) | Outcome unknown; **never mark a payout failed**. Query by `merchantOrderNo`, or resend the identical request under the same number |
| `*APIError` | The gateway returned a business error (`Msg` / `Message` / `TraceID`) | Branch on `msg`. `IDEMPOTENCY_CONFLICT`: the number is already in flight, query it and keep querying rather than switching numbers. `CHANNEL_ERROR`: the order may already exist, query by `merchantOrderNo` first |
| `*ResponseError` | The gateway or CDN returned something that is not an envelope (HTML 502, ...) | Outcome unknown; query before deciding |
| `errors.Is(err, ErrResponseTooLarge)` | A response arrived but exceeded the size limit and was discarded | Outcome unknown; the order was most likely created, query before deciding |
| Anything else | An unexpected error; assume the request may have arrived | Outcome unknown; query before deciding |

**`merchantOrderNo` is the only key that prevents a duplicate order.** A second
create with the same number never creates a second order: the platform answers
with the original order, or with `IDEMPOTENCY_CONFLICT` while the first one is
still being placed. The idempotency key travels with the request for tracing and
is **not** a deduplication key.

Two rules follow:

- After an unknown outcome, never allocate a new `merchantOrderNo`. Query the
  existing one, or resend the same request under the same number.
- A resend must carry identical parameters. The platform returns the original
  order without comparing fields, so a changed amount or account silently has no
  effect. To change anything, use a new `merchantOrderNo` and reconcile the
  original order first.

The SDK validates locally before signing (top-level required fields and formats,
method shape and required extras); the rules are defined in
[`protocol/merchant-api.md`](protocol/merchant-api.md#client-side-validation).
Format checks (phone length, e-mail, …) stay with the gateway on purpose.

## 2. Idempotency and retries

Write calls are not retried automatically. On a timeout, recover by querying the
order via `merchantOrderNo` or `orderNo`. Retry safety comes from reusing the
same `merchantOrderNo`, not from the idempotency key: the key is carried for
tracing only. The SDK regenerates the request nonce on every attempt, so calling
the same method again is a valid retry while replaying captured bytes is not.

```go
order, err := c.CreatePayment(ctx, req, joogopay.WithIdempotencyKey(key))
```

## 3. Webhook

```go
wh, err := c.ParsePaymentWebhook(r)  // strict orderType=PAYMENT; mismatch -> ErrInvalidWebhookBody
// or
wh, err := c.ParsePayoutWebhook(r)   // strict orderType=PAYOUT
```

Verification order: nil check -> shape -> Content-Digest over the body ->
`Webhook-Event-Id` matches the body `eventId` -> platform Ed25519 signature by
the `keyid` in `Signature-Input`. Signatures are fresh for a short window, so the
platform re-signs every delivery attempt. Deduplicate delivery side effects by
`eventId`, make order state updates
idempotent by `orderNo` or `merchantOrderNo`, and return 2xx on success or the
platform retries.

### Failure fields (present when `status=FAILED`)

Both webhooks and order queries return failure. Branch on `failure.msg`;
`failure.message` is for display and troubleshooting only.

```go
if wh.Status == "FAILED" && wh.Failure != nil {
    switch wh.Failure.Msg {
    case joogopay.MsgChannelError:        // the order may already exist; requery by merchantOrderNo, do not reissue
    case joogopay.MsgOrderRejected:       // risk/business rejection
    case joogopay.MsgInsufficientBalance:
    }
}
```

Full error-code table: [`protocol/errors.md`](protocol/errors.md).

## 4. Amounts

`Amount`, `PaidAmount`, balances, fees and rates are decimal strings such as
`"100.50"`, never JSON numbers. Keep amounts as strings end to end; do not parse
them into float64.

## 5. Unlisted methods

```go
m := joogopay.PaymentMethod{Code: "NEW_METHOD"}
_ = m.SetExtra("newMethod", map[string]any{"customerName": "X", "bankCode": "001"})
```


## 6. Protocol and test vectors

[`protocol/`](protocol/) is the cross-language source of truth: the
signature wire spec, the sealed-box envelope spec, and fixed test vectors.

## 7. Development

```sh
go test -race ./...       # full test suite
```
