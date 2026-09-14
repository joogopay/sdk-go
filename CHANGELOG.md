# Changelog — go

Versions follow SemVer. Tags are `vX.Y.Z` on this repository.

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
