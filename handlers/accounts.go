package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/Craiglowdon/eagle-bank-api/middleware"
	"github.com/Craiglowdon/eagle-bank-api/models"
)

type AccountHandler struct {
	db *sql.DB
}

func NewAccountHandler(db *sql.DB) *AccountHandler {
	return &AccountHandler{
		db: db,
	}
}

func (h *AccountHandler) CreateAccount(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.AuthenticatedUserID(r.Context())
	if !ok {
		response := models.ErrorResponse{
			Message: "access token is missing or invalid",
		}

		_ = writeJSON(w, http.StatusUnauthorized, response)
		return
	}

	var request models.CreateBankAccountRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response := models.BadRequestErrorResponse{
			Message: "invalid request body",
			Details: []models.ValidationErrorDetail{},
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	if details := validateCreateAccountRequest(request); len(details) > 0 {
		response := models.BadRequestErrorResponse{
			Message: "invalid request",
			Details: details,
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	accountNumber, err := generateAccountNumber()
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to create account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	now := time.Now().UTC()

	account := models.BankAccount{
		AccountNumber:    accountNumber,
		SortCode:         "10-10-10",
		Name:             request.Name,
		AccountType:      request.AccountType,
		Balance:          0,
		Currency:         models.CurrencyGBP,
		CreatedTimestamp: now,
		UpdatedTimestamp: now,
	}

	_, err = h.db.ExecContext(
		r.Context(),
		`
			INSERT INTO accounts (
				account_number,
				user_id,
				sort_code,
				name,
				account_type,
				balance_pence,
				currency,
				created_timestamp,
				updated_timestamp
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
		account.AccountNumber,
		userID,
		account.SortCode,
		account.Name,
		string(account.AccountType),
		0,
		string(account.Currency),
		account.CreatedTimestamp.Format(time.RFC3339Nano),
		account.UpdatedTimestamp.Format(time.RFC3339Nano),
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to create account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	_ = writeJSON(w, http.StatusCreated, account)
}

func validateCreateAccountRequest(
	request models.CreateBankAccountRequest,
) []models.ValidationErrorDetail {
	var details []models.ValidationErrorDetail

	if strings.TrimSpace(request.Name) == "" {
		details = append(details, models.ValidationErrorDetail{
			Field:   "name",
			Message: "name is required",
			Type:    "required",
		})
	}

	if request.AccountType == "" {
		details = append(details, models.ValidationErrorDetail{
			Field:   "accountType",
			Message: "accountType is required",
			Type:    "required",
		})
	} else if request.AccountType != models.AccountTypePersonal {
		details = append(details, models.ValidationErrorDetail{
			Field:   "accountType",
			Message: "accountType must be personal",
			Type:    "enum",
		})
	}

	return details
}

func generateAccountNumber() (string, error) {
	number, err := rand.Int(
		rand.Reader,
		big.NewInt(1_000_000),
	)
	if err != nil {
		return "", fmt.Errorf("generate random number: %w", err)
	}

	return fmt.Sprintf("01%06d", number.Int64()), nil
}

func (h *AccountHandler) ListAccounts(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.AuthenticatedUserID(r.Context())
	if !ok {
		response := models.ErrorResponse{
			Message: "access token is missing or invalid",
		}

		_ = writeJSON(w, http.StatusUnauthorized, response)
		return
	}

	rows, err := h.db.QueryContext(
		r.Context(),
		`
			SELECT
				account_number,
				user_id,
				sort_code,
				name,
				account_type,
				balance_pence,
				currency,
				created_timestamp,
				updated_timestamp
			FROM accounts
			WHERE user_id = ?
			ORDER BY created_timestamp, account_number
		`,
		userID,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to list accounts",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}
	defer rows.Close()

	accounts := make([]models.BankAccount, 0)

	for rows.Next() {
		storedAccount, err := scanBankAccount(rows)
		if err != nil {
			response := models.ErrorResponse{
				Message: "failed to list accounts",
			}

			_ = writeJSON(
				w,
				http.StatusInternalServerError,
				response,
			)
			return
		}

		accounts = append(accounts, storedAccount.account)
	}

	if err := rows.Err(); err != nil {
		response := models.ErrorResponse{
			Message: "failed to list accounts",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	response := models.ListBankAccountsResponse{
		Accounts: accounts,
	}

	_ = writeJSON(w, http.StatusOK, response)
}

type rowScanner interface {
	Scan(dest ...any) error
}

type storedBankAccount struct {
	account models.BankAccount
	userID  string
}

func scanBankAccount(
	row rowScanner,
) (storedBankAccount, error) {
	var storedAccount storedBankAccount
	var accountType string
	var balancePence int64
	var currency string
	var createdTimestamp string
	var updatedTimestamp string

	err := row.Scan(
		&storedAccount.account.AccountNumber,
		&storedAccount.userID,
		&storedAccount.account.SortCode,
		&storedAccount.account.Name,
		&accountType,
		&balancePence,
		&currency,
		&createdTimestamp,
		&updatedTimestamp,
	)
	if err != nil {
		return storedBankAccount{},
			fmt.Errorf("scan account: %w", err)
	}

	storedAccount.account.AccountType = models.AccountType(
		accountType,
	)
	storedAccount.account.Balance = float64(balancePence) / 100
	storedAccount.account.Currency = models.Currency(currency)

	storedAccount.account.CreatedTimestamp, err = time.Parse(
		time.RFC3339Nano,
		createdTimestamp,
	)
	if err != nil {
		return storedBankAccount{},
			fmt.Errorf("parse created timestamp: %w", err)
	}

	storedAccount.account.UpdatedTimestamp, err = time.Parse(
		time.RFC3339Nano,
		updatedTimestamp,
	)
	if err != nil {
		return storedBankAccount{},
			fmt.Errorf("parse updated timestamp: %w", err)
	}

	return storedAccount, nil
}

func (h *AccountHandler) GetAccount(
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

	storedAccount, err := scanBankAccount(
		h.db.QueryRowContext(
			r.Context(),
			`
				SELECT
					account_number,
					user_id,
					sort_code,
					name,
					account_type,
					balance_pence,
					currency,
					created_timestamp,
					updated_timestamp
				FROM accounts
				WHERE account_number = ?
			`,
			accountNumber,
		),
	)

	if errors.Is(err, sql.ErrNoRows) {
		response := models.ErrorResponse{
			Message: "account not found",
		}

		_ = writeJSON(w, http.StatusNotFound, response)
		return
	}

	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to fetch account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if storedAccount.userID != authenticatedUserID {
		response := models.ErrorResponse{
			Message: "you are not allowed to access this account",
		}

		_ = writeJSON(w, http.StatusForbidden, response)
		return
	}

	_ = writeJSON(w, http.StatusOK, storedAccount.account)
}

func (h *AccountHandler) UpdateAccount(
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

	var request models.UpdateBankAccountRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response := models.BadRequestErrorResponse{
			Message: "invalid request body",
			Details: []models.ValidationErrorDetail{},
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	accountNumber := r.PathValue("accountNumber")

	storedAccount, err := scanBankAccount(
		h.db.QueryRowContext(
			r.Context(),
			`
				SELECT
					account_number,
					user_id,
					sort_code,
					name,
					account_type,
					balance_pence,
					currency,
					created_timestamp,
					updated_timestamp
				FROM accounts
				WHERE account_number = ?
			`,
			accountNumber,
		),
	)

	if errors.Is(err, sql.ErrNoRows) {
		response := models.ErrorResponse{
			Message: "account not found",
		}

		_ = writeJSON(w, http.StatusNotFound, response)
		return
	}

	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to update account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if storedAccount.userID != authenticatedUserID {
		response := models.ErrorResponse{
			Message: "you are not allowed to update this account",
		}

		_ = writeJSON(w, http.StatusForbidden, response)
		return
	}

	if request.Name == nil && request.AccountType == nil {
		_ = writeJSON(
			w,
			http.StatusOK,
			storedAccount.account,
		)
		return
	}

	updatedAccount := storedAccount.account

	if request.Name != nil {
		updatedAccount.Name = *request.Name
	}

	if request.AccountType != nil {
		updatedAccount.AccountType = *request.AccountType
	}

	validationRequest := models.CreateBankAccountRequest{
		Name:        updatedAccount.Name,
		AccountType: updatedAccount.AccountType,
	}

	if details := validateCreateAccountRequest(
		validationRequest,
	); len(details) > 0 {
		response := models.BadRequestErrorResponse{
			Message: "invalid request",
			Details: details,
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	updatedAccount.UpdatedTimestamp = time.Now().UTC()

	_, err = h.db.ExecContext(
		r.Context(),
		`
			UPDATE accounts
			SET
				name = ?,
				account_type = ?,
				updated_timestamp = ?
			WHERE account_number = ?
		`,
		updatedAccount.Name,
		string(updatedAccount.AccountType),
		updatedAccount.UpdatedTimestamp.Format(time.RFC3339Nano),
		accountNumber,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to update account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	_ = writeJSON(w, http.StatusOK, updatedAccount)
}

func (h *AccountHandler) DeleteAccount(
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

	databaseTransaction, err := h.db.BeginTx(
		r.Context(),
		nil,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to delete account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}
	defer databaseTransaction.Rollback()

	var accountOwnerID string

	err = databaseTransaction.QueryRowContext(
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
			Message: "failed to delete account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if accountOwnerID != authenticatedUserID {
		response := models.ErrorResponse{
			Message: "you are not allowed to delete this account",
		}

		_ = writeJSON(w, http.StatusForbidden, response)
		return
	}

	var transactionCount int

	err = databaseTransaction.QueryRowContext(
		r.Context(),
		`
			SELECT COUNT(*)
			FROM transactions
			WHERE account_number = ?
		`,
		accountNumber,
	).Scan(&transactionCount)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to delete account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if transactionCount > 0 {
		response := models.ErrorResponse{
			Message: "account cannot be deleted while it has transactions",
		}

		_ = writeJSON(w, http.StatusConflict, response)
		return
	}

	_, err = databaseTransaction.ExecContext(
		r.Context(),
		`
			DELETE FROM accounts
			WHERE account_number = ?
		`,
		accountNumber,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to delete account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if err := databaseTransaction.Commit(); err != nil {
		response := models.ErrorResponse{
			Message: "failed to delete account",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
