package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/Craiglowdon/eagle-bank-api/middleware"
	"github.com/Craiglowdon/eagle-bank-api/models"
)

const maximumBalancePence int64 = 1_000_000

type TransactionHandler struct {
	db *sql.DB
}

func NewTransactionHandler(db *sql.DB) *TransactionHandler {
	return &TransactionHandler{
		db: db,
	}
}

func (h *TransactionHandler) CreateTransaction(
	w http.ResponseWriter,
	r *http.Request,
) {
	authenticatedUserID, ok := middleware.AuthenticatedUserID(
		r.Context(),
	)
	if !ok {
		response := models.ErrorResponse{
			Message: "access token is missing or invalid",
		}

		_ = writeJSON(w, http.StatusUnauthorized, response)
		return
	}

	var request models.CreateTransactionRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response := models.BadRequestErrorResponse{
			Message: "invalid request body",
			Details: []models.ValidationErrorDetail{},
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	amountPence, validationErrors :=
		validateCreateTransactionRequest(request)

	if len(validationErrors) > 0 {
		response := models.BadRequestErrorResponse{
			Message: "invalid request",
			Details: validationErrors,
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	accountNumber := r.PathValue("accountNumber")

	databaseTransaction, err := h.db.BeginTx(
		r.Context(),
		nil,
	)
	if err != nil {
		h.writeTransactionError(w)
		return
	}
	defer databaseTransaction.Rollback()

	var accountOwnerID string
	var currentBalancePence int64

	err = databaseTransaction.QueryRowContext(
		r.Context(),
		`
			SELECT user_id, balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		accountNumber,
	).Scan(
		&accountOwnerID,
		&currentBalancePence,
	)

	if errors.Is(err, sql.ErrNoRows) {
		response := models.ErrorResponse{
			Message: "account not found",
		}

		_ = writeJSON(w, http.StatusNotFound, response)
		return
	}

	if err != nil {
		h.writeTransactionError(w)
		return
	}

	if accountOwnerID != authenticatedUserID {
		response := models.ErrorResponse{
			Message: "you are not allowed to access this account",
		}

		_ = writeJSON(w, http.StatusForbidden, response)
		return
	}

	var updatedBalancePence int64

	switch request.Type {
	case models.TransactionTypeDeposit:
		if amountPence > maximumBalancePence-currentBalancePence {
			response := models.ErrorResponse{
				Message: "transaction would exceed maximum balance",
			}

			_ = writeJSON(
				w,
				http.StatusUnprocessableEntity,
				response,
			)
			return
		}

		updatedBalancePence = currentBalancePence + amountPence

	case models.TransactionTypeWithdrawal:
		if amountPence > currentBalancePence {
			response := models.ErrorResponse{
				Message: "insufficient funds",
			}

			_ = writeJSON(
				w,
				http.StatusUnprocessableEntity,
				response,
			)
			return
		}

		updatedBalancePence = currentBalancePence - amountPence
	}

	transactionID, err := generateTransactionID()
	if err != nil {
		h.writeTransactionError(w)
		return
	}

	now := time.Now().UTC()

	createdTransaction := models.Transaction{
		ID:               transactionID,
		Amount:           float64(amountPence) / 100,
		Currency:         request.Currency,
		Type:             request.Type,
		Reference:        request.Reference,
		UserID:           authenticatedUserID,
		CreatedTimestamp: now,
	}

	_, err = databaseTransaction.ExecContext(
		r.Context(),
		`
			UPDATE accounts
			SET
				balance_pence = ?,
				updated_timestamp = ?
			WHERE account_number = ?
		`,
		updatedBalancePence,
		now.Format(time.RFC3339Nano),
		accountNumber,
	)
	if err != nil {
		h.writeTransactionError(w)
		return
	}

	_, err = databaseTransaction.ExecContext(
		r.Context(),
		`
			INSERT INTO transactions (
				id,
				account_number,
				user_id,
				amount_pence,
				currency,
				transaction_type,
				reference,
				created_timestamp
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
		createdTransaction.ID,
		accountNumber,
		createdTransaction.UserID,
		amountPence,
		string(createdTransaction.Currency),
		string(createdTransaction.Type),
		createdTransaction.Reference,
		createdTransaction.CreatedTimestamp.Format(time.RFC3339Nano),
	)
	if err != nil {
		h.writeTransactionError(w)
		return
	}

	if err := databaseTransaction.Commit(); err != nil {
		h.writeTransactionError(w)
		return
	}

	_ = writeJSON(
		w,
		http.StatusCreated,
		createdTransaction,
	)
}

func validateCreateTransactionRequest(
	request models.CreateTransactionRequest,
) (int64, []models.ValidationErrorDetail) {
	var details []models.ValidationErrorDetail
	var amountPence int64

	if request.Amount == nil {
		details = append(details, models.ValidationErrorDetail{
			Field:   "amount",
			Message: "amount is required",
			Type:    "required",
		})
	} else {
		var err error

		amountPence, err = convertAmountToPence(*request.Amount)
		if err != nil {
			details = append(details, models.ValidationErrorDetail{
				Field:   "amount",
				Message: err.Error(),
				Type:    "invalid",
			})
		}
	}

	if request.Currency == "" {
		details = append(details, models.ValidationErrorDetail{
			Field:   "currency",
			Message: "currency is required",
			Type:    "required",
		})
	} else if request.Currency != models.CurrencyGBP {
		details = append(details, models.ValidationErrorDetail{
			Field:   "currency",
			Message: "currency must be GBP",
			Type:    "enum",
		})
	}

	if request.Type == "" {
		details = append(details, models.ValidationErrorDetail{
			Field:   "type",
			Message: "type is required",
			Type:    "required",
		})
	} else if request.Type != models.TransactionTypeDeposit &&
		request.Type != models.TransactionTypeWithdrawal {
		details = append(details, models.ValidationErrorDetail{
			Field:   "type",
			Message: "type must be deposit or withdrawal",
			Type:    "enum",
		})
	}

	return amountPence, details
}

func convertAmountToPence(amount float64) (int64, error) {
	if amount < 0 || amount > 10_000 {
		return 0, fmt.Errorf(
			"amount must be between 0 and 10000",
		)
	}

	scaledAmount := amount * 100
	roundedAmount := math.Round(scaledAmount)

	if math.Abs(scaledAmount-roundedAmount) > 0.000001 {
		return 0, fmt.Errorf(
			"amount must have no more than two decimal places",
		)
	}

	return int64(roundedAmount), nil
}

func generateTransactionID() (string, error) {
	bytes := make([]byte, 8)

	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf(
			"generate transaction ID: %w",
			err,
		)
	}

	return "tan-" + hex.EncodeToString(bytes), nil
}

func (h *TransactionHandler) writeTransactionError(
	w http.ResponseWriter,
) {
	response := models.ErrorResponse{
		Message: "failed to create transaction",
	}

	_ = writeJSON(
		w,
		http.StatusInternalServerError,
		response,
	)
}

func scanTransaction(
	row rowScanner,
) (models.Transaction, error) {
	var transaction models.Transaction
	var amountPence int64
	var currency string
	var transactionType string
	var reference sql.NullString
	var createdTimestamp string

	err := row.Scan(
		&transaction.ID,
		&amountPence,
		&currency,
		&transactionType,
		&reference,
		&transaction.UserID,
		&createdTimestamp,
	)
	if err != nil {
		return models.Transaction{},
			fmt.Errorf("scan transaction: %w", err)
	}

	transaction.Amount = float64(amountPence) / 100
	transaction.Currency = models.Currency(currency)
	transaction.Type = models.TransactionType(transactionType)
	transaction.Reference = reference.String

	transaction.CreatedTimestamp, err = time.Parse(
		time.RFC3339Nano,
		createdTimestamp,
	)
	if err != nil {
		return models.Transaction{},
			fmt.Errorf("parse created timestamp: %w", err)
	}

	return transaction, nil
}

func (h *TransactionHandler) ListTransactions(
	w http.ResponseWriter,
	r *http.Request,
) {
	authenticatedUserID, ok := middleware.AuthenticatedUserID(
		r.Context(),
	)
	if !ok {
		response := models.ErrorResponse{
			Message: "access token is missing or invalid",
		}

		_ = writeJSON(w, http.StatusUnauthorized, response)
		return
	}

	accountNumber := r.PathValue("accountNumber")

	var accountOwnerID string

	err := h.db.QueryRowContext(
		r.Context(),
		`
			SELECT user_id
			FROM accounts
			WHERE account_number = ?
		`,
		accountNumber,
	).Scan(&accountOwnerID)

	if errors.Is(err, sql.ErrNoRows) {
		response := models.ErrorResponse{
			Message: "account not found",
		}

		_ = writeJSON(w, http.StatusNotFound, response)
		return
	}

	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to list transactions",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if accountOwnerID != authenticatedUserID {
		response := models.ErrorResponse{
			Message: "you are not allowed to access this account",
		}

		_ = writeJSON(w, http.StatusForbidden, response)
		return
	}

	rows, err := h.db.QueryContext(
		r.Context(),
		`
			SELECT
				id,
				amount_pence,
				currency,
				transaction_type,
				reference,
				user_id,
				created_timestamp
			FROM transactions
			WHERE account_number = ?
			ORDER BY created_timestamp, id
		`,
		accountNumber,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to list transactions",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}
	defer rows.Close()

	transactions := make([]models.Transaction, 0)

	for rows.Next() {
		transaction, err := scanTransaction(rows)
		if err != nil {
			response := models.ErrorResponse{
				Message: "failed to list transactions",
			}

			_ = writeJSON(
				w,
				http.StatusInternalServerError,
				response,
			)
			return
		}

		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		response := models.ErrorResponse{
			Message: "failed to list transactions",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	response := models.ListTransactionsResponse{
		Transactions: transactions,
	}

	_ = writeJSON(w, http.StatusOK, response)
}

func (h *TransactionHandler) GetTransaction(
	w http.ResponseWriter,
	r *http.Request,
) {
	authenticatedUserID, ok := middleware.AuthenticatedUserID(
		r.Context(),
	)
	if !ok {
		response := models.ErrorResponse{
			Message: "access token is missing or invalid",
		}

		_ = writeJSON(w, http.StatusUnauthorized, response)
		return
	}

	accountNumber := r.PathValue("accountNumber")
	transactionID := r.PathValue("transactionId")

	var accountOwnerID string

	err := h.db.QueryRowContext(
		r.Context(),
		`
			SELECT user_id
			FROM accounts
			WHERE account_number = ?
		`,
		accountNumber,
	).Scan(&accountOwnerID)

	if errors.Is(err, sql.ErrNoRows) {
		response := models.ErrorResponse{
			Message: "account not found",
		}

		_ = writeJSON(w, http.StatusNotFound, response)
		return
	}

	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to fetch transaction",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if accountOwnerID != authenticatedUserID {
		response := models.ErrorResponse{
			Message: "you are not allowed to access this account",
		}

		_ = writeJSON(w, http.StatusForbidden, response)
		return
	}

	transaction, err := scanTransaction(
		h.db.QueryRowContext(
			r.Context(),
			`
				SELECT
					id,
					amount_pence,
					currency,
					transaction_type,
					reference,
					user_id,
					created_timestamp
				FROM transactions
				WHERE id = ?
				  AND account_number = ?
			`,
			transactionID,
			accountNumber,
		),
	)

	if errors.Is(err, sql.ErrNoRows) {
		response := models.ErrorResponse{
			Message: "transaction not found",
		}

		_ = writeJSON(w, http.StatusNotFound, response)
		return
	}

	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to fetch transaction",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	_ = writeJSON(w, http.StatusOK, transaction)
}
