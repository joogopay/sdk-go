# JooGoPay SDK — go

商户开放 API 的 Go SDK。[English](./README.md) | **简体中文**

请求用商户 Ed25519 私钥按 RFC 9421 HTTP Message Signatures 签名；POST 请求体用平台
X25519 公钥做 libsodium sealed box 加密。签名、摘要、加密由 SDK 处理。
`NewClient` 拒绝非 HTTPS BaseURL。同步响应仍是明文 JSON，其保密性和完整性由 HTTPS/TLS
保证。

## 0. 安装

```
go get github.com/joogopay/sdk-go
```

需要 Go 1.24 及以上。唯一的第三方依赖是 `golang.org/x/crypto`。

## 1. 用法

```go
import joogopay "github.com/joogopay/sdk-go"

c, err := joogopay.NewClient(joogopay.Config{
    BaseURL:                     "https://panama.joogopay.com",     // 生产环境域名；不带 /api/v1
    AccessKey:                   "mak_live_xxx",
    MerchantPrivateKeyBase64:    merchantPrivateKey,            // 商户 Ed25519 私钥，base64
    PlatformBodyKeyID:           "body_20260827_01",
    PlatformBodyPublicKeyBase64: platformBodyPublicKey,         // 平台 X25519 公钥，base64
    PlatformWebhookPublicKeys:   platformWebhookPublicKeys,     // keyID -> base64 Ed25519 公钥
})
if err != nil { return err }

order, err := c.CreatePayment(ctx, &joogopay.CreatePaymentReq{
    MerchantOrderNo: "M20260101001",
    Currency:        joogopay.CurrencyBRL,
    Amount:          "100.00",                                  // decimal 字符串，不是 JSON number
    PaymentMethod: joogopay.PaymentMethod{
        Code: joogopay.MethodCodePIX,
        Pix:  &joogopay.PaymentPixExtra{PayerName: "Joao Silva"},
    },
    WebhookUrl: "https://merchant.example/webhook/payments",
})

// 错误处理
var apiErr *joogopay.APIError
var respErr *joogopay.ResponseError
switch {
case errors.As(err, &apiErr):  // 业务错误（envelope 已解析）
    log.Printf("api error: %s %s (trace=%s)", apiErr.Msg, apiErr.Message, apiErr.TraceID)
case errors.As(err, &respErr): // 基础设施错误（网关/CDN 返回非 envelope）
case err != nil:               // transport 错误
}
```

密钥由商户负责：商户自行生成 Ed25519 密钥对，只把公钥交给平台，平台不接收、不保存私钥。

### ARS 代收与代付

ARS 代收分别使用 `BANK_TRANSFER` / `BankTransfer`、`CVU` / `Cvu`、`QRIS` / `Qris`。
三种方式都需要 `FirstName`、`LastName`、`Email`、`DocumentType`（DNI/CUIT）和
`DocumentNumber`。CVU、QRIS 还必须提供 `Phone`：首位非 0 的 10 位数字字符串。
BANK_TRANSFER 不要求手机号。请求示例中的身份与账户资料均为虚构，接入时替换为实际资料。

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

将付款人跳转到 `order.Action.Url` 的渠道收银台。三种方式独立选择，不自动切换；
打开链接或浏览器回跳不表示到账，最终结果以查询或验签后的平台 Webhook 为准。

ARS 代付只使用 `BANK_TRANSFER`，`AccountType` 必须为字符串 `"CBU"` 或 `"CVU"`；
`AccountNo` 使用数字字符串保留前导零。手机号格式与代收相同，其余八个收款字段必填，`Address` 可选。
缺省、`null` 和空字符串均表示无地址，非空字符串原样保留，数字、布尔值、数组和对象会被拒绝。
Go 类型化字段为空时省略；`SetExtra` 可以显式传入 `null` 或空字符串。`DocumentType` 和 `DocumentNumber` 不能为空：

```go
order, err := c.CreatePayout(ctx, &joogopay.CreatePayoutReq{
    MerchantOrderNo: "ars-payout-demo-001", Currency: joogopay.CurrencyARS, Amount: "1000.00",
    PayoutMethod: joogopay.PayoutMethod{
        Code: joogopay.MethodCodeBankTransfer,
        BankTransfer: &joogopay.PayoutBankTransferExtra{
            FirstName: "Ana", LastName: "Perez", Email: "ana@example.com", Phone: "1123456789",
            Address: "Av Example 123", DocumentType: "DNI", DocumentNumber: "30123456",
            AccountNo: "0000003100012345678901", AccountType: "CBU", // 也可使用 "CVU"
        },
    },
    WebhookUrl: "https://merchant.example.com/webhook",
})
```

SDK 在发送前检查必填字段；手机号格式与账户类型取值由网关校验。其他币种沿用各自的账户类型。

### PEN 代收与代付

[可执行 PEN 示例](pen_example_test.go)使用现有类型构建三种代收请求
（`BANK_TRANSFER`、`E_WALLET`、`CASH`）及银行、Yape、Plin 三种代付请求。
示例只序列化请求，不发送网络请求，可运行 `go test -run 'ExampleCreate.*Req_pen'` 验证。

钱包代付使用 `E_WALLET` / `EWallet`，`BankCode` 为 `026` 表示 Yape，为 `025` 表示
Plin；`AccountNo` 是钱包收款标识，`CustomerPhone` 是联系电话。银行代付使用
`BANK_TRANSFER` / `BankTransfer`，提供 `SAVINGS` 或 `CHECKING` 的 `AccountType`
及 20 位 `CciNo`。账号保持字符串。示例中的身份及账户资料均为虚构，实际请求前必须替换；
支付方式是否可用以商户配置为准。

### 错误分两类，处置相反

| 错误 | 含义 | 处置 |
| --- | --- | --- |
| `errors.Is(err, ErrInvalidRequest)` | 请求**没发出去**（本地校验失败、参数非法，或 SDK 无法编码、签名） | 可以安全判负，修参数后用同一个 `merchantOrderNo` 重试 |
| `errors.Is(err, ErrTransport)` | 请求已交给传输层但没拿到可用响应（连接失败、超时、读响应中断） | 结果不确定，**出款一律不得判负**。用 `merchantOrderNo` 查询，或用同一个单号原样重发 |
| `*APIError` | 网关返回了业务错误（`Msg` / `Message` / `TraceID`） | 按 `Msg` 分支。`IDEMPOTENCY_CONFLICT`：该单号已被占用但平台取不回那笔订单，继续按该单号查询，不要换号。`CHANNEL_ERROR`：订单可能已生成，先用 `merchantOrderNo` 查询，只有查询返回 `ORDER_NOT_FOUND` 才可以用同一单号重新下单。`CHANNEL_BUSY`：网关在建单前就拒了，退避后用同一单号直接重发，这是唯一不需要先查单的渠道错误 |
| `*ResponseError` | 网关 / CDN 返回了非 envelope 响应（HTML 502 等） | 结果不确定，查询后再判定 |
| `errors.Is(err, ErrResponseTooLarge)` | 响应已收到但超过大小上限被丢弃 | 结果不确定，订单大概率已创建，查询后再判定 |
| 其他 | 意料之外的错误，按请求可能已到达处理 | 结果不确定，查询后再判定 |

**`merchantOrderNo` 是唯一能防止重复下单的键。** 同一个单号第二次创建不会产生第二笔订单：平台返回原订单，或在认定该单号已被占用却取不回那笔订单时返回 `IDEMPOTENCY_CONFLICT`。idempotency key 只随请求用于链路追踪，**不是**去重键。

由此有两条规则：

- 结果未知时绝不换新的 `merchantOrderNo`。查询原单，或用同一个单号原样重发。
- 重发必须参数完全一致。平台返回原订单时不比对字段，改过的金额或收款账号不会生效。要改任何参数，必须换新单号，并先处理掉原订单。

SDK 在签名前会做本地校验（顶层必填与格式、方式码结构与必填），规则以
[`protocol/merchant-api.md`](protocol/merchant-api.md#client-side-validation) 为准；
手机号位数、邮箱等格式校验故意留给网关，避免 SDK 与网关漂移。

## 2. 幂等与重试

写请求不自动重试。超时后用 `merchantOrderNo` 或 `orderNo` 查询恢复。重试安全性来自复用同一个
`merchantOrderNo`，不是来自 idempotency key：该键只用于链路追踪。SDK 每次请求重新生成 nonce，
所以重新调用同一个方法是有效的重试，重放抓包的字节则不是。

```go
order, err := c.CreatePayment(ctx, req, joogopay.WithIdempotencyKey(key))
```

## 3. Webhook

```go
wh, err := c.ParsePaymentWebhook(r)  // 严格 orderType=PAYMENT；不符 -> ErrInvalidWebhookBody
// 或
wh, err := c.ParsePayoutWebhook(r)   // 严格 orderType=PAYOUT
```

验证顺序：nil 检查 -> shape -> Content-Digest 校验 body -> `Webhook-Event-Id` 与 body
`eventId` 一致 -> 用 `Signature-Input` 里 `keyid` 对应的平台 Ed25519 公钥验签。签名有效期
是短窗口，平台每次投递都会重新签名。按 `eventId` 做投递去重，按 `orderNo` 或
`merchantOrderNo` 对订单状态更新做业务幂等。成功返回 2xx，否则平台重试。

### Failure 字段（`status=FAILED` 时存在）

webhook 与订单查询响应都会返回 failure。用 `failure.msg` 做稳定分支，`failure.message`
只用于展示或排障。

```go
if wh.Status == "FAILED" && wh.Failure != nil {
    switch wh.Failure.Msg {
    case joogopay.MsgChannelError:        // 订单可能已生成，用 merchantOrderNo 查询，不要换新号重发
    case joogopay.MsgOrderRejected:       // 风控/业务拒绝
    case joogopay.MsgInsufficientBalance:
    }
}
```

完整错误码表见 [`protocol/errors.md`](protocol/errors.md)。

## 4. 金额

`Amount`、`PaidAmount`、余额、手续费、费率都是 `"100.50"` 这样的 decimal 字符串，不是 JSON
number。全链路保持字符串，不要解析成 float64。

## 5. 未发版方法接入

```go
m := joogopay.PaymentMethod{Code: "NEW_METHOD"}
_ = m.SetExtra("newMethod", map[string]any{"customerName": "X", "bankCode": "001"})
```

强类型 `PaymentNewMethodExtra` 会在后续 minor 版本补上。

## 6. 协议与测试向量

[`protocol/`](protocol/) 是跨语言真相源：签名 wire 规范、sealed box envelope 规范、
固定测试向量。

## 7. 开发

```sh
go test -race ./...       # 全套测试
```
