package models

import "time"

type TransactionType string

const (
	TransactionTypeDeposit    TransactionType = "deposit"
	TransactionTypeWithdrawal TransactionType = "withdrawal"
)

type CreateTransactionRequest struct {
	Amount    *float64        `json:"amount"`
	Currency  Currency        `json:"currency"`
	Type      TransactionType `json:"type"`
	Reference string          `json:"reference,omitempty"`
}

type Transaction struct {
	ID               string          `json:"id"`
	Amount           float64         `json:"amount"`
	Currency         Currency        `json:"currency"`
	Type             TransactionType `json:"type"`
	Reference        string          `json:"reference,omitempty"`
	UserID           string          `json:"userId"`
	CreatedTimestamp time.Time       `json:"createdTimestamp"`
}

type ListTransactionsResponse struct {
	Transactions []Transaction `json:"transactions"`
}
