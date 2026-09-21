# Changelog — go

Versions follow SemVer. Tags are `vX.Y.Z` on this repository.

## v0.4.1 — 2026-09-21

- Method-code allowlists now cover every currency and direction the gateway
  validates, so a code the gateway would refuse is rejected before sending as
  "method not available" instead of coming back as an API error. Newly listed:
  pay-in `ARS` (`BANK_TRANSFER`, `CVU`, `QRIS`), `BRL` (`PIX`), `CLP` (`KHIPU`,
  `MACH`, `PAGO46`, `WEBPAY`), `COP` (`BREB`, `NEQUI`, `PSE`), `MXN` (`CASH`,
  `OXXO`, `SPEI`), `TRY` (`BANK_TRANSFER`); payout `ARS`, `CLP`, `MXN`
  (`BANK_TRANSFER`), `BRL` (`PIX`), `COP` (`BANK_CARD`, `BANK_TRANSFER`, `BREB`,
  `TRANSFIYA`), `IDR` (`ID_BANK_TRANSFER` and the five wallets), `TRY`
  (`BANK_TRANSFER`, `PAPARA`). Types and constants are unchanged.

## v0.4.0 — 2026-09-19

- `PayoutMethod` gains typed `PhGcash` / `PhMaya` fields for Philippine payouts.
  `PhDfWallet` stays for every other wallet, GrabPay included. The `PHP` payout
  allowlist adds `PH_GCASH` / `PH_MAYA`, and `bankCode` is required only for
  `PH_DF_BANK` and `PH_DF_WALLET`; the channel derives it for the named wallets.

## v0.3.1 — 2026-09-18

- Needs a platform that accepts an omitted or `null` ARS `address` (platform
  release of 2026-09-18); against an earlier platform, send `address` as a string.
- ARS `BANK_TRANSFER` payout `address` is optional. Omitted, `null` and empty
  strings mean no address; non-empty strings are preserved. Other value types
  are rejected before sending. The other eight recipient fields remain required,
  and other currencies and methods retain their existing rules.
- Empty typed `Address` values are omitted instead of forcing an ARS empty
  string onto the wire. `SetExtra` preserves explicit `null` and empty strings.

## v0.3.0 — 2026-09-18

- ARS payouts accept an empty `Address`. It is the only required field that may
  be blank, and it must still reach the gateway as a string: for `ARS` /
  `BANK_TRANSFER` the SDK now sends `"address": ""` instead of dropping the empty
  field, while a missing, `null` or non-string `address` given through
  `SetExtra` is still rejected before the request goes out. v0.2.0 rejected an
  empty address as a missing required field.
- USD payments accept `CASH_APP` only; USD payouts accept `CASH_APP`, `PAYPAL`
  and `CHIME`. Each carries its own required set, checked before the request
  goes out; v0.2.0 had no USD rules and passed every USD request through to the
  gateway. `PayoutMethod` gains typed `PayPal` / `Chime` fields,
  `PayoutCashAppExtra` (shared by the three payout methods) gains `FirstName`,
  `LastName`, `DateOfBirth`, `CountryOfResidence`, `StateOfResidence`,
  `CardCity`, `CardStreet` and `CardPostCode`, and `MethodCodePayPal` /
  `MethodCodeChime` are new constants.
- Documentation: executable PEN payment and payout examples in
  `pen_example_test.go`; expanded godoc for `Config`; `IDEMPOTENCY_CONFLICT`
  and `CHANNEL_BUSY` handling in the error table; `protocol/` links in the
  README are absolute so they resolve on pkg.go.dev.

## v0.2.0 — 2026-09-14

- `PayoutMethod` gains typed `IdDana` / `IdOvo` / `IdGopay` / `IdLinkaja` /
  `IdShopeepay` fields (`*PayoutBankAccountContactExtra`) for Indonesia wallet payouts.
  The other language SDKs take the payout method as a map and need no change.

## v0.1.0 — 2026-09-14

Initial public release.

- Ed25519 request signing (RFC 9421 HTTP Message Signatures) and X25519
  sealed-box body encryption for POST requests.
- Platform webhook verification: `VerifyWebhook`, `ParsePaymentWebhook`, `ParsePayoutWebhook`.
- Endpoints: `CreatePayment`, `CreatePayout`, `QueryPaymentByOrderNo`, `QueryPaymentByMerchantOrderNo`, `QueryPayoutByOrderNo`, `QueryPayoutByMerchantOrderNo`, `GetBalance`, `GetUSDRate`, `GetPayoutReceipt`, `GetPaymentCheckout`, `SubmitPaymentTradeNo`, `AddPaymentExtraInfo`.
- Local validation before signing: top-level required fields, decimal-string
  amount within `DECIMAL(18,2)`, `https` webhook URL, method-code shape and
  per-currency required extras. Format checks (phone, e-mail, IFSC) stay with
  the gateway.
- Error taxonomy: `ErrInvalidRequest` (rejected before sending), `ErrTransport` (sent or possibly sent, no response), `*APIError` (gateway business error), `*ResponseError` (non-envelope response).
- Amounts, balances, fees and rates are decimal strings; idempotency key
  support via `WithIdempotencyKey`; typed method extras with `SetExtra` for methods not yet typed.
