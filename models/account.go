package models

import "time"

type AccountType string

const (
	AccountTypePersonal AccountType = "personal"
)

type Currency string

const (
	CurrencyGBP Currency = "GBP"
)

type CreateBankAccountRequest struct {
	Name        string      `json:"name"`
	AccountType AccountType `json:"accountType"`
}

type UpdateBankAccountRequest struct {
	Name        *string      `json:"name"`
	AccountType *AccountType `json:"accountType"`
}

type BankAccount struct {
	AccountNumber    string      `json:"accountNumber"`
	SortCode         string      `json:"sortCode"`
	Name             string      `json:"name"`
	AccountType      AccountType `json:"accountType"`
	Balance          float64     `json:"balance"`
	Currency         Currency    `json:"currency"`
	CreatedTimestamp time.Time   `json:"createdTimestamp"`
	UpdatedTimestamp time.Time   `json:"updatedTimestamp"`
}

type ListBankAccountsResponse struct {
	Accounts []BankAccount `json:"accounts"`
}
