// Code generated from the JSON rule data under protocol/data. DO NOT EDIT.

package joogopay

import "regexp"

// Top-level create-order field rules, from protocol/data/request-rules.json.
var (
	createRequiredTextFields = []string{"merchantOrderNo", "currency"}
	amountPattern            = regexp.MustCompile(`^(0|[1-9][0-9]{0,15})(\.[0-9]{1,2})?$`)
	webhookURLPrefix         = "https://"
)

// methodExtraFields maps a method code to its extra field name in paymentMethod / payoutMethod.
var methodExtraFields = map[string]string{
	"APPLE_PAY":         "applePay",
	"BANK_CARD":         "bankCard",
	"BANK_TRANSFER":     "bankTransfer",
	"BD_BKASH":          "bdBkash",
	"BD_NAGAD":          "bdNagad",
	"BREB":              "breb",
	"CASH":              "cash",
	"CASH_APP":          "cashApp",
	"CHIME":             "chime",
	"CREDIT_CARD":       "creditCard",
	"CVU":               "cvu",
	"E_WALLET":          "eWallet",
	"GOOGLE_PAY":        "googlePay",
	"ID_BANK_TRANSFER":  "idBankTransfer",
	"ID_DANA":           "idDana",
	"ID_GOPAY":          "idGopay",
	"ID_LINKAJA":        "idLinkaja",
	"ID_OVO":            "idOvo",
	"ID_QRIS":           "idQris",
	"ID_SHOPEEPAY":      "idShopeepay",
	"ID_VA":             "idVa",
	"IN_IFSC":           "inIfsc",
	"IN_UPI":            "inUpi",
	"KHIPU":             "khipu",
	"MACH":              "mach",
	"NEQUI":             "nequi",
	"NETELLER":          "neteller",
	"P2P":               "p2p",
	"PAGO46":            "pago46",
	"PAGO_FACIL":        "pagoFacil",
	"PAPARA":            "papara",
	"PAYPAL":            "paypal",
	"PH_DF_BANK":        "phDfBank",
	"PH_DF_WALLET":      "phDfWallet",
	"PH_GCASH":          "phGcash",
	"PH_GCASH_QR":       "phGcashQr",
	"PH_GRAB":           "phGrab",
	"PH_MAYA":           "phMaya",
	"PH_MAYA_QR":        "phMayaQr",
	"PH_NATIVE_GCASH":   "phNativeGcash",
	"PH_QRIS":           "phQris",
	"PIX":               "pix",
	"PK_BANK":           "pkBank",
	"PK_EASYPAISA":      "pkEasypaisa",
	"PK_EASYPAISA_QRPH": "pkEasypaisaQrph",
	"PK_JAZZCASH":       "pkJazzcash",
	"PK_JAZZCASH_QRPH":  "pkJazzcashQrph",
	"PSE":               "pse",
	"QRIS":              "qris",
	"RAPIPAGO":          "rapipago",
	"SBP":               "sbp",
	"SERVIFACIL":        "serviFacil",
	"SKRILL":            "skrill",
	"SPEI":              "spei",
	"TH_BANK_CARD":      "thBankCard",
	"TH_BANK_TRANSFER":  "thBankTransfer",
	"TH_PROMPTPAY":      "thPromptpay",
	"TH_TRUEMONEY":      "thTruemoney",
	"TRANSFIYA":         "transfiya",
	"USDT-BEP20":        "usdtBep20",
	"USDT-ERC20":        "usdtErc20",
	"USDT-TRC20":        "usdtTrc20",
	"WEBPAY":            "webpay",
}

// methodRule is the method-code allowlist and required fields of one currency in one direction.
type methodRule struct {
	codes      []string            // empty means the gateway has no allowlist; the SDK does not reject on code
	required   []string            // always required
	byMethod   map[string][]string // extra fields required by that method code
	allowEmpty []string            // required strings that may be empty
}

var paymentMethodRules = map[string]methodRule{
	"ARS": {required: []string{"documentNumber", "documentType", "email", "firstName", "lastName"}, byMethod: map[string][]string{"CVU": {"phone"}, "QRIS": {"phone"}}},
	"BDT": {codes: []string{"BD_BKASH", "BD_NAGAD"}, required: []string{"accountName", "email", "mobile"}},
	"BRL": {},
	"CLP": {required: []string{"customerEmail", "customerName", "documentNumber", "documentType"}},
	"COP": {byMethod: map[string][]string{"BREB": {"customerEmail", "customerName", "customerPhone", "documentNumber", "documentType"}}},
	"IDR": {codes: []string{"ID_DANA", "ID_GOPAY", "ID_LINKAJA", "ID_OVO", "ID_QRIS", "ID_SHOPEEPAY", "ID_VA"}, required: []string{"accountName", "bankCode", "email", "mobile"}},
	"INR": {codes: []string{"IN_UPI"}, required: []string{"accountName", "email", "mobile"}},
	"MXN": {},
	"PEN": {codes: []string{"BANK_TRANSFER", "CASH", "E_WALLET"}, required: []string{"customerEmail", "customerName", "customerPhone", "documentNumber", "documentType"}},
	"PHP": {codes: []string{"PH_GCASH", "PH_GCASH_QR", "PH_GRAB", "PH_MAYA", "PH_MAYA_QR", "PH_NATIVE_GCASH", "PH_QRIS"}},
	"PKR": {codes: []string{"PK_EASYPAISA", "PK_EASYPAISA_QRPH", "PK_JAZZCASH", "PK_JAZZCASH_QRPH"}},
	"TRY": {required: []string{"customerName"}},
	"USD": {codes: []string{"CASH_APP"}, required: []string{"name", "phone", "email", "ipAddress"}},
}

var payoutMethodRules = map[string]methodRule{
	"ARS": {required: []string{"accountNo", "accountType", "address", "documentNumber", "documentType", "email", "firstName", "lastName", "phone"}, allowEmpty: []string{"address"}},
	"BDT": {codes: []string{"BD_BKASH", "BD_NAGAD"}, required: []string{"accountName", "accountNo", "email", "mobile"}},
	"BRL": {required: []string{"key", "keyType"}},
	"CLP": {required: []string{"accountName", "accountNo", "accountType", "bankCode", "customerEmail", "customerPhone", "documentNumber", "documentType"}},
	"COP": {required: []string{"customerEmail", "customerName", "customerPhone", "documentNumber", "documentType"}, byMethod: map[string][]string{"BANK_CARD": {"accountNo", "bankName"}, "BANK_TRANSFER": {"accountNo", "bankName"}, "BREB": {"accountNo"}}},
	"IDR": {required: []string{"accountName", "bankCode", "email", "mobile"}},
	"INR": {codes: []string{"IN_IFSC", "IN_UPI"}, required: []string{"email", "mobile", "name"}, byMethod: map[string][]string{"IN_IFSC": {"account", "ifsc"}}},
	"MXN": {required: []string{"accountName", "accountNo", "accountType", "bankCode", "bankName"}},
	"PEN": {codes: []string{"BANK_TRANSFER", "E_WALLET"}, required: []string{"accountName", "accountNo", "bankCode", "customerEmail", "customerPhone", "documentNumber", "documentType"}, byMethod: map[string][]string{"BANK_TRANSFER": {"accountType", "cciNo"}}},
	"PHP": {codes: []string{"PH_DF_BANK", "PH_DF_WALLET"}, required: []string{"accountName", "accountNo", "bankCode", "email", "mobile"}},
	"PKR": {codes: []string{"PK_BANK", "PK_EASYPAISA", "PK_JAZZCASH"}, required: []string{"accountNo", "cnic", "mobile"}, byMethod: map[string][]string{"PK_BANK": {"bankCode"}}},
	"TRY": {required: []string{"accountName", "accountNo"}, byMethod: map[string][]string{"BANK_TRANSFER": {"bankCode", "bankName"}}},
	"USD": {codes: []string{"CASH_APP", "PAYPAL", "CHIME"}, required: []string{"name", "phone", "email", "accountNo", "firstName", "lastName", "dateOfBirth", "countryOfResidence", "stateOfResidence", "cardCity", "cardStreet", "cardPostCode"}},
}
