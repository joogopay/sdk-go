package joogopay

import (
	"encoding/json"
	"strings"
)

type CreatePaymentReq struct {
	MerchantOrderNo string        `json:"merchantOrderNo"`
	Currency        string        `json:"currency"`
	Amount          string        `json:"amount"`
	Country         string        `json:"country,omitempty"`
	PaymentMethod   PaymentMethod `json:"paymentMethod"`
	ReturnUrl       string        `json:"returnUrl,omitempty"`
	WebhookUrl      string        `json:"webhookUrl"`
	Attach          string        `json:"attach,omitempty"`
}

type CreatePayoutReq struct {
	MerchantOrderNo string       `json:"merchantOrderNo"`
	Currency        string       `json:"currency"`
	Amount          string       `json:"amount"`
	Country         string       `json:"country,omitempty"`
	PayoutMethod    PayoutMethod `json:"payoutMethod"`
	WebhookUrl      string       `json:"webhookUrl"`
	Attach          string       `json:"attach,omitempty"`
}

// PaymentMethod holds strongly-typed fields for every current payment method branch.
// For a method not yet in a release, wire it at runtime via SetExtra("xxx", payload).
type PaymentMethod struct {
	Code            string                               `json:"code"`
	Pix             *PaymentPixExtra                     `json:"pix,omitempty"`
	PagoFacil       *PaymentArsDocumentExtra             `json:"pagoFacil,omitempty"`
	Rapipago        *PaymentArsDocumentExtra             `json:"rapipago,omitempty"`
	BankTransfer    *PaymentBankTransferExtra            `json:"bankTransfer,omitempty"`
	Cvu             *PaymentArsDocumentExtra             `json:"cvu,omitempty"`
	Qris            *PaymentArsQrisExtra                 `json:"qris,omitempty"`
	Spei            *PaymentSpeiExtra                    `json:"spei,omitempty"`
	Cash            *PaymentCustomerDocumentContactExtra `json:"cash,omitempty"`
	Pse             *PaymentPseExtra                     `json:"pse,omitempty"`
	Nequi           *PaymentNequiExtra                   `json:"nequi,omitempty"`
	Transfiya       *PaymentCustomerDocumentContactExtra `json:"transfiya,omitempty"`
	Breb            *PaymentCustomerDocumentContactExtra `json:"breb,omitempty"`
	Webpay          *PaymentCustomerDocumentEmailExtra   `json:"webpay,omitempty"`
	Khipu           *PaymentCustomerDocumentEmailExtra   `json:"khipu,omitempty"`
	Mach            *PaymentCustomerDocumentEmailExtra   `json:"mach,omitempty"`
	Pago46          *PaymentCustomerDocumentEmailExtra   `json:"pago46,omitempty"`
	ServiFacil      *PaymentCustomerDocumentEmailExtra   `json:"serviFacil,omitempty"`
	EWallet         *PaymentEWalletExtra                 `json:"eWallet,omitempty"`
	CashApp         *PaymentUsCustomerExtra              `json:"cashApp,omitempty"`
	CreditCard      *PaymentUsCustomerExtra              `json:"creditCard,omitempty"`
	ApplePay        *PaymentUsCustomerExtra              `json:"applePay,omitempty"`
	GooglePay       *PaymentUsCustomerExtra              `json:"googlePay,omitempty"`
	Neteller        *PaymentUsCustomerExtra              `json:"neteller,omitempty"`
	Skrill          *PaymentUsCustomerExtra              `json:"skrill,omitempty"`
	UsdtTrc20       *PaymentUsdtExtra                    `json:"usdtTrc20,omitempty"`
	UsdtErc20       *PaymentUsdtExtra                    `json:"usdtErc20,omitempty"`
	UsdtBep20       *PaymentUsdtExtra                    `json:"usdtBep20,omitempty"`
	BdBkash         *PaymentBdWalletExtra                `json:"bdBkash,omitempty"`
	BdNagad         *PaymentBdWalletExtra                `json:"bdNagad,omitempty"`
	IdVa            *PaymentBankAccountContactExtra      `json:"idVa,omitempty"`
	IdQris          *PaymentBankAccountContactExtra      `json:"idQris,omitempty"`
	IdDana          *PaymentBankAccountContactExtra      `json:"idDana,omitempty"`
	IdOvo           *PaymentBankAccountContactExtra      `json:"idOvo,omitempty"`
	IdGopay         *PaymentBankAccountContactExtra      `json:"idGopay,omitempty"`
	IdLinkaja       *PaymentBankAccountContactExtra      `json:"idLinkaja,omitempty"`
	IdShopeepay     *PaymentBankAccountContactExtra      `json:"idShopeepay,omitempty"`
	PhGcash         *PaymentAccountContactExtra          `json:"phGcash,omitempty"`
	PhMaya          *PaymentAccountContactExtra          `json:"phMaya,omitempty"`
	PhGrab          *PaymentAccountContactExtra          `json:"phGrab,omitempty"`
	PhQris          *PaymentAccountContactExtra          `json:"phQris,omitempty"`
	PhGcashQr       *PaymentAccountContactExtra          `json:"phGcashQr,omitempty"`
	PhMayaQr        *PaymentAccountContactExtra          `json:"phMayaQr,omitempty"`
	PhNativeGcash   *PaymentAccountContactExtra          `json:"phNativeGcash,omitempty"`
	PkJazzcash      *PaymentPkWalletExtra                `json:"pkJazzcash,omitempty"`
	PkEasypaisa     *PaymentPkWalletExtra                `json:"pkEasypaisa,omitempty"`
	PkJazzcashQrph  *PaymentPkWalletExtra                `json:"pkJazzcashQrph,omitempty"`
	PkEasypaisaQrph *PaymentPkWalletExtra                `json:"pkEasypaisaQrph,omitempty"`
	ThBankCard      *PaymentBankAccountContactExtra      `json:"thBankCard,omitempty"`
	ThTruemoney     *PaymentBankAccountContactExtra      `json:"thTruemoney,omitempty"`
	ThPromptpay     *PaymentBankAccountContactExtra      `json:"thPromptpay,omitempty"`
	InUpi           *PaymentAccountContactExtra          `json:"inUpi,omitempty"`

	extra map[string]any
}

// SetExtra injects the payload for an unlisted method, overriding a same-named typed field.
// It returns ErrInvalidExtraField when field is empty or "code".
func (m *PaymentMethod) SetExtra(field string, value any) error {
	field = strings.TrimSpace(field)
	if field == "" || field == "code" {
		return ErrInvalidExtraField
	}
	if m.extra == nil {
		m.extra = map[string]any{}
	}
	m.extra[field] = value
	return nil
}

func (m PaymentMethod) MarshalJSON() ([]byte, error) {
	type alias PaymentMethod
	b, err := json.Marshal(alias(m))
	if err != nil {
		return nil, err
	}
	if len(m.extra) == 0 {
		return b, nil
	}
	obj := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, err
	}
	for k, v := range m.extra {
		eb, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		obj[k] = eb
	}
	return json.Marshal(obj)
}

type PaymentPixExtra struct {
	PayerCPF  string `json:"payerCPF,omitempty"`
	PayerName string `json:"payerName,omitempty"`
	CpfVerify *bool  `json:"cpfVerify,omitempty"`
}

type PaymentArsDocumentExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	FirstName      string `json:"firstName,omitempty"`
	LastName       string `json:"lastName,omitempty"`
	Email          string `json:"email,omitempty"`
	Phone          string `json:"phone,omitempty"` // Required for ARS CVU.
}

type PaymentArsQrisExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	FirstName      string `json:"firstName,omitempty"`
	LastName       string `json:"lastName,omitempty"`
	Email          string `json:"email,omitempty"`
	Phone          string `json:"phone,omitempty"`
}

type PaymentBankTransferExtra struct {
	CustomerId     string `json:"customerId,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CustomerPhone  string `json:"customerPhone,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	FirstName      string `json:"firstName,omitempty"`
	LastName       string `json:"lastName,omitempty"`
	Email          string `json:"email,omitempty"`
}

type PaymentCustomerDocumentContactExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CustomerPhone  string `json:"customerPhone,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
}

type PaymentCustomerDocumentEmailExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
}

// PaymentSpeiExtra holds the MXN SPEI pay-in extra fields.
//
// The amount mode is set by MinAmount / MaxAmount, orthogonal to AllowMultiplePayments:
//   - both 0: fixed amount; the settled amount equals CreatePaymentReq.Amount.
//   - both > 0: range amount; the settled amount is reported in PaidAmount.
//
// AllowMultiplePayments controls whether one pay-in entry accepts multiple valid payments (default false).
type PaymentSpeiExtra struct {
	AllowMultiplePayments bool   `json:"allowMultiplePayments,omitempty"`
	MinAmount             string `json:"minAmount,omitempty"`
	MaxAmount             string `json:"maxAmount,omitempty"`
}

type PaymentPseExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
	CustomerPhone  string `json:"customerPhone,omitempty"`
	BankCode       string `json:"bankCode,omitempty"`
}

type PaymentNequiExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
	CustomerPhone  string `json:"customerPhone,omitempty"`
	AccountNo      string `json:"accountNo,omitempty"`
}

type PaymentEWalletExtra struct {
	CustomerId     string `json:"customerId,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CustomerPhone  string `json:"customerPhone,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
}

type PaymentUsCustomerExtra struct {
	Name      string `json:"name,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	IpAddress string `json:"ipAddress,omitempty"`
}

type PaymentUsdtExtra struct {
	CustomerId string `json:"customerId,omitempty"`
}

type PaymentBdWalletExtra struct {
	PayType     string `json:"payType,omitempty"`
	AccountName string `json:"accountName,omitempty"`
	Email       string `json:"email,omitempty"`
	Mobile      string `json:"mobile,omitempty"`
}

type PaymentBankAccountContactExtra struct {
	AccountName string `json:"accountName,omitempty"`
	Email       string `json:"email,omitempty"`
	Mobile      string `json:"mobile,omitempty"`
	BankCode    string `json:"bankCode,omitempty"`
	AccountNo   string `json:"accountNo,omitempty"`
}

type PaymentAccountContactExtra struct {
	AccountName string `json:"accountName,omitempty"`
	Email       string `json:"email,omitempty"`
	Mobile      string `json:"mobile,omitempty"`
}

type PaymentPkWalletExtra struct {
	AccountNo   string `json:"accountNo,omitempty"`
	AccountName string `json:"accountName,omitempty"`
	Email       string `json:"email,omitempty"`
	Mobile      string `json:"mobile,omitempty"`
	AutoFill    string `json:"autoFill,omitempty"`
	Direct      string `json:"direct,omitempty"`
}

// PayoutMethod holds strongly-typed fields for every current payout branch;
// like PaymentMethod, use SetExtra for an unlisted method.
type PayoutMethod struct {
	Code           string                         `json:"code"`
	Pix            *PayoutPixExtra                `json:"pix,omitempty"`
	BankTransfer   *PayoutBankTransferExtra       `json:"bankTransfer,omitempty"`
	Breb           *PayoutColombiaBankExtra       `json:"breb,omitempty"`
	BankCard       *PayoutColombiaBankExtra       `json:"bankCard,omitempty"`
	Transfiya      *PayoutTransfiyaExtra          `json:"transfiya,omitempty"`
	Papara         *PayoutPaparaExtra             `json:"papara,omitempty"`
	CashApp        *PayoutCashAppExtra            `json:"cashApp,omitempty"`
	UsdtTrc20      *PayoutUsdtExtra               `json:"usdtTrc20,omitempty"`
	UsdtErc20      *PayoutUsdtExtra               `json:"usdtErc20,omitempty"`
	UsdtBep20      *PayoutUsdtExtra               `json:"usdtBep20,omitempty"`
	P2P            *PayoutRubBankExtra            `json:"p2p,omitempty"`
	Sbp            *PayoutRubBankExtra            `json:"sbp,omitempty"`
	BdBkash        *PayoutAccountContactExtra     `json:"bdBkash,omitempty"`
	BdNagad        *PayoutAccountContactExtra     `json:"bdNagad,omitempty"`
	IdBankTransfer *PayoutBankAccountContactExtra `json:"idBankTransfer,omitempty"`
	PhDfWallet     *PayoutBankAccountContactExtra `json:"phDfWallet,omitempty"`
	PhDfBank       *PayoutBankAccountContactExtra `json:"phDfBank,omitempty"`
	EWallet        *PayoutEWalletExtra            `json:"eWallet,omitempty"`
	PkJazzcash     *PayoutPkDfExtra               `json:"pkJazzcash,omitempty"`
	PkEasypaisa    *PayoutPkDfExtra               `json:"pkEasypaisa,omitempty"`
	PkBank         *PayoutPkDfExtra               `json:"pkBank,omitempty"`
	ThBankTransfer *PayoutBankAccountContactExtra `json:"thBankTransfer,omitempty"`
	InIfsc         *PayoutInIfscExtra             `json:"inIfsc,omitempty"`
	InUpi          *PayoutInUpiExtra              `json:"inUpi,omitempty"`

	extra map[string]any
}

func (m *PayoutMethod) SetExtra(field string, value any) error {
	field = strings.TrimSpace(field)
	if field == "" || field == "code" {
		return ErrInvalidExtraField
	}
	if m.extra == nil {
		m.extra = map[string]any{}
	}
	m.extra[field] = value
	return nil
}

func (m PayoutMethod) MarshalJSON() ([]byte, error) {
	type alias PayoutMethod
	b, err := json.Marshal(alias(m))
	if err != nil {
		return nil, err
	}
	if len(m.extra) == 0 {
		return b, nil
	}
	obj := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, err
	}
	for k, v := range m.extra {
		eb, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		obj[k] = eb
	}
	return json.Marshal(obj)
}

type PayoutPixExtra struct {
	KeyType     string `json:"keyType,omitempty"`
	Key         string `json:"key,omitempty"`
	Document    string `json:"document,omitempty"`
	AccountName string `json:"accountName,omitempty"`
	CpfVerify   *bool  `json:"cpfVerify,omitempty"`
	AccountType string `json:"accountType,omitempty"`
	BankBranch  string `json:"bankBranch,omitempty"`
}

type PayoutBankTransferExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	FirstName      string `json:"firstName,omitempty"`
	LastName       string `json:"lastName,omitempty"`
	Name           string `json:"name,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CustomerPhone  string `json:"customerPhone,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
	Phone          string `json:"phone,omitempty"`
	Email          string `json:"email,omitempty"`
	Address        string `json:"address,omitempty"`
	AccountName    string `json:"accountName,omitempty"`
	AccountNo      string `json:"accountNo,omitempty"`
	AccountType    string `json:"accountType,omitempty"`
	BankCode       string `json:"bankCode,omitempty"`
	BankName       string `json:"bankName,omitempty"`
	CciNo          string `json:"cciNo,omitempty"`
}

type PayoutColombiaBankExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CustomerPhone  string `json:"customerPhone,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
	AccountNo      string `json:"accountNo,omitempty"`
	BankName       string `json:"bankName,omitempty"`
	BankCode       string `json:"bankCode,omitempty"`
	AccountType    string `json:"accountType,omitempty"`
}

type PayoutTransfiyaExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CustomerPhone  string `json:"customerPhone,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
}

type PayoutPaparaExtra struct {
	AccountName string `json:"accountName,omitempty"`
	AccountNo   string `json:"accountNo,omitempty"`
}

type PayoutCashAppExtra struct {
	Name      string `json:"name,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	AccountNo string `json:"accountNo,omitempty"`
}

type PayoutUsdtExtra struct {
	CryptoAddress string `json:"cryptoAddress,omitempty"`
	CustomerId    string `json:"customerId,omitempty"`
}

type PayoutRubBankExtra struct {
	Name      string `json:"name,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	BankCode  string `json:"bankCode,omitempty"`
	AccountNo string `json:"accountNo,omitempty"`
}

type PayoutAccountContactExtra struct {
	AccountNo   string `json:"accountNo,omitempty"`
	AccountName string `json:"accountName,omitempty"`
	Email       string `json:"email,omitempty"`
	Mobile      string `json:"mobile,omitempty"`
}

type PayoutBankAccountContactExtra struct {
	AccountNo   string `json:"accountNo,omitempty"`
	BankCode    string `json:"bankCode,omitempty"`
	AccountName string `json:"accountName,omitempty"`
	Email       string `json:"email,omitempty"`
	Mobile      string `json:"mobile,omitempty"`
}

type PayoutPkDfExtra struct {
	AccountNo   string `json:"accountNo,omitempty"`
	Cnic        string `json:"cnic,omitempty"`
	BankCode    string `json:"bankCode,omitempty"`
	AccountName string `json:"accountName,omitempty"`
	Email       string `json:"email,omitempty"`
	Mobile      string `json:"mobile,omitempty"`
}

type PayoutEWalletExtra struct {
	DocumentType   string `json:"documentType,omitempty"`
	DocumentNumber string `json:"documentNumber,omitempty"`
	CustomerPhone  string `json:"customerPhone,omitempty"`
	CustomerEmail  string `json:"customerEmail,omitempty"`
	AccountName    string `json:"accountName,omitempty"`
	AccountNo      string `json:"accountNo,omitempty"`
	BankCode       string `json:"bankCode,omitempty"`
}

type PayoutInIfscExtra struct {
	Account string `json:"account,omitempty"`
	Ifsc    string `json:"ifsc,omitempty"`
	Name    string `json:"name,omitempty"`
	Email   string `json:"email,omitempty"`
	Mobile  string `json:"mobile,omitempty"`
}

type PayoutInUpiExtra struct {
	Account string `json:"account,omitempty"`
	Name    string `json:"name,omitempty"`
	Email   string `json:"email,omitempty"`
	Mobile  string `json:"mobile,omitempty"`
}

type OrderAction struct {
	Url string `json:"url,omitempty"`
	// Payment content for merchant-built checkout. See the corresponding currency
	// and payment method page for the format.
	PayContent string `json:"payContent,omitempty"`
	QrCode     string `json:"qrCode,omitempty"`
}

type PaymentOrder struct {
	OrderNo         string          `json:"orderNo"`
	MerchantOrderNo string          `json:"merchantOrderNo"`
	Status          string          `json:"status"`
	Currency        string          `json:"currency"`
	Amount          string          `json:"amount"`
	PaidAmount      string          `json:"paidAmount"`
	Country         string          `json:"country,omitempty"`
	PaymentMethod   string          `json:"paymentMethod"`
	Action          OrderAction     `json:"action"`
	Attach          string          `json:"attach,omitempty"`
	Failure         *WebhookFailure `json:"failure,omitempty"`
	CreatedAt       int64           `json:"createdAt"`
	UpdatedAt       int64           `json:"updatedAt"`
}

type PayoutOrder struct {
	OrderNo         string          `json:"orderNo"`
	MerchantOrderNo string          `json:"merchantOrderNo"`
	Status          string          `json:"status"`
	Currency        string          `json:"currency"`
	Amount          string          `json:"amount"`
	Country         string          `json:"country,omitempty"`
	PayoutMethod    string          `json:"payoutMethod"`
	Action          OrderAction     `json:"action"`
	Attach          string          `json:"attach,omitempty"`
	Failure         *WebhookFailure `json:"failure,omitempty"`
	CreatedAt       int64           `json:"createdAt"`
	UpdatedAt       int64           `json:"updatedAt"`
}

type ReceiptBank struct {
	Ispb    string `json:"ispb,omitempty"`
	Name    string `json:"name,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Account string `json:"account,omitempty"`
	Number  string `json:"number,omitempty"`
}

type ReceiptAccount struct {
	Bank    *ReceiptBank `json:"bank,omitempty"`
	Name    string       `json:"name,omitempty"`
	TaxId   string       `json:"taxId,omitempty"`
	TaxType string       `json:"taxType,omitempty"`
	Key     string       `json:"key,omitempty"`
}

type PayoutReceipt struct {
	OrderNo            string          `json:"orderNo"`
	Amount             string          `json:"amount"`
	Currency           string          `json:"currency"`
	Timestamp          int64           `json:"timestamp"`
	ChannelTradeNo     string          `json:"channelTradeNo,omitempty"`
	SourceAccount      *ReceiptAccount `json:"sourceAccount,omitempty"`
	DestinationAccount *ReceiptAccount `json:"destinationAccount,omitempty"`
	Url                string          `json:"url,omitempty"`
}

type Balance struct {
	Currency           string `json:"currency"`
	Balance            string `json:"balance"`
	LockBalance        string `json:"lockBalance"`
	PaymentBalance     string `json:"paymentBalance"`
	PaymentLockBalance string `json:"paymentLockBalance"`
	PayoutBalance      string `json:"payoutBalance"`
	PayoutLockBalance  string `json:"payoutLockBalance"`
}

type USDRate struct {
	UsdRate string `json:"usdRate"`
}
