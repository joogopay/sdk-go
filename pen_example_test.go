package joogopay

import (
	"encoding/json"
	"fmt"
)

// These examples only build requests; they do not send payments.
// Replace all fictional identity and account details before using a real client.
func ExampleCreatePaymentReq_pen() {
	methods := []PaymentMethod{
		{
			Code: MethodCodeBankTransfer,
			BankTransfer: &PaymentBankTransferExtra{
				CustomerName: "Test Payer", CustomerPhone: "912345678", CustomerEmail: "payer@example.test",
				DocumentType: "DNI", DocumentNumber: "12345678",
			},
		},
		{
			Code: MethodCodeEWallet,
			EWallet: &PaymentEWalletExtra{
				CustomerName: "Test Payer", CustomerPhone: "912345678", CustomerEmail: "payer@example.test",
				DocumentType: "DNI", DocumentNumber: "12345678",
			},
		},
		{
			Code: MethodCodeCash,
			Cash: &PaymentCustomerDocumentContactExtra{
				CustomerName: "Test Payer", CustomerPhone: "912345678", CustomerEmail: "payer@example.test",
				DocumentType: "DNI", DocumentNumber: "12345678",
			},
		},
	}
	for i, method := range methods {
		req := CreatePaymentReq{
			MerchantOrderNo: fmt.Sprintf("pen-payment-example-%d", i+1),
			Currency:        CurrencyPEN, Country: "PE", Amount: "10.25",
			PaymentMethod: method,
			ReturnUrl:     "https://merchant.example.test/return",
			WebhookUrl:    "https://merchant.example.test/webhook/payment",
		}
		body, err := json.Marshal(req)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(body))
	}
	// After creation, redirect the payer to order.Action.Url.
	// Opening or returning from that page does not confirm payment.

	// Output:
	// {"merchantOrderNo":"pen-payment-example-1","currency":"PEN","amount":"10.25","country":"PE","paymentMethod":{"code":"BANK_TRANSFER","bankTransfer":{"customerName":"Test Payer","customerPhone":"912345678","customerEmail":"payer@example.test","documentType":"DNI","documentNumber":"12345678"}},"returnUrl":"https://merchant.example.test/return","webhookUrl":"https://merchant.example.test/webhook/payment"}
	// {"merchantOrderNo":"pen-payment-example-2","currency":"PEN","amount":"10.25","country":"PE","paymentMethod":{"code":"E_WALLET","eWallet":{"customerName":"Test Payer","customerPhone":"912345678","customerEmail":"payer@example.test","documentType":"DNI","documentNumber":"12345678"}},"returnUrl":"https://merchant.example.test/return","webhookUrl":"https://merchant.example.test/webhook/payment"}
	// {"merchantOrderNo":"pen-payment-example-3","currency":"PEN","amount":"10.25","country":"PE","paymentMethod":{"code":"CASH","cash":{"documentType":"DNI","documentNumber":"12345678","customerName":"Test Payer","customerPhone":"912345678","customerEmail":"payer@example.test"}},"returnUrl":"https://merchant.example.test/return","webhookUrl":"https://merchant.example.test/webhook/payment"}
}

func ExampleCreatePayoutReq_pen() {
	methods := []PayoutMethod{
		{
			Code: MethodCodeBankTransfer,
			BankTransfer: &PayoutBankTransferExtra{
				DocumentType: "DNI", DocumentNumber: "12345678",
				CustomerPhone: "912345678", CustomerEmail: "recipient@example.test",
				AccountName: "Test Recipient", AccountNo: "00123456789",
				AccountType: "SAVINGS", BankCode: "002", CciNo: "00212345678901234567",
			},
		},
		{
			Code: MethodCodeEWallet,
			EWallet: &PayoutEWalletExtra{
				DocumentType: "DNI", DocumentNumber: "12345678",
				CustomerPhone: "912345678", CustomerEmail: "recipient@example.test",
				AccountName: "Test Recipient", AccountNo: "912345678", BankCode: "026", // Yape
			},
		},
		{
			Code: MethodCodeEWallet,
			EWallet: &PayoutEWalletExtra{
				DocumentType: "DNI", DocumentNumber: "12345678",
				CustomerPhone: "912345678", CustomerEmail: "recipient@example.test",
				AccountName: "Test Recipient", AccountNo: "912345678", BankCode: "025", // Plin
			},
		},
	}
	for i, method := range methods {
		req := CreatePayoutReq{
			MerchantOrderNo: fmt.Sprintf("pen-payout-example-%d", i+1),
			Currency:        CurrencyPEN, Country: "PE", Amount: "10.25",
			PayoutMethod: method,
			WebhookUrl:   "https://merchant.example.test/webhook/payout",
		}
		body, err := json.Marshal(req)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(body))
	}
	// Account numbers and the bank CCI remain strings to preserve leading zeros.
	// Wallet accountNo identifies the recipient; customerPhone is contact information.

	// Output:
	// {"merchantOrderNo":"pen-payout-example-1","currency":"PEN","amount":"10.25","country":"PE","payoutMethod":{"code":"BANK_TRANSFER","bankTransfer":{"documentType":"DNI","documentNumber":"12345678","customerPhone":"912345678","customerEmail":"recipient@example.test","accountName":"Test Recipient","accountNo":"00123456789","accountType":"SAVINGS","bankCode":"002","cciNo":"00212345678901234567"}},"webhookUrl":"https://merchant.example.test/webhook/payout"}
	// {"merchantOrderNo":"pen-payout-example-2","currency":"PEN","amount":"10.25","country":"PE","payoutMethod":{"code":"E_WALLET","eWallet":{"documentType":"DNI","documentNumber":"12345678","customerPhone":"912345678","customerEmail":"recipient@example.test","accountName":"Test Recipient","accountNo":"912345678","bankCode":"026"}},"webhookUrl":"https://merchant.example.test/webhook/payout"}
	// {"merchantOrderNo":"pen-payout-example-3","currency":"PEN","amount":"10.25","country":"PE","payoutMethod":{"code":"E_WALLET","eWallet":{"documentType":"DNI","documentNumber":"12345678","customerPhone":"912345678","customerEmail":"recipient@example.test","accountName":"Test Recipient","accountNo":"912345678","bankCode":"025"}},"webhookUrl":"https://merchant.example.test/webhook/payout"}
}
