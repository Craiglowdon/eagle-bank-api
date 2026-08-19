package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Craiglowdon/eagle-bank-api/models"
)

func TestCreateDeposit(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	amount := 10.99

	requestBody := models.CreateTransactionRequest{
		Amount:    &amount,
		Currency:  models.CurrencyGBP,
		Type:      models.TransactionTypeDeposit,
		Reference: "Birthday money",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to encode transaction request: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/accounts/"+account.AccountNumber+"/transactions",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	var transaction models.Transaction

	if err := json.NewDecoder(response.Body).Decode(
		&transaction,
	); err != nil {
		t.Fatalf("failed to decode transaction response: %v", err)
	}

	if !strings.HasPrefix(transaction.ID, "tan-") {
		t.Errorf(
			"expected transaction ID to start with tan-, got %q",
			transaction.ID,
		)
	}

	if transaction.Amount != *requestBody.Amount {
		t.Errorf(
			"expected amount %v, got %v",
			*requestBody.Amount,
			transaction.Amount,
		)
	}

	if transaction.Currency != models.CurrencyGBP {
		t.Errorf(
			"expected currency %q, got %q",
			models.CurrencyGBP,
			transaction.Currency,
		)
	}

	if transaction.Type != models.TransactionTypeDeposit {
		t.Errorf(
			"expected transaction type %q, got %q",
			models.TransactionTypeDeposit,
			transaction.Type,
		)
	}

	if transaction.Reference != requestBody.Reference {
		t.Errorf(
			"expected reference %q, got %q",
			requestBody.Reference,
			transaction.Reference,
		)
	}

	if transaction.UserID != createdUser.ID {
		t.Errorf(
			"expected user ID %q, got %q",
			createdUser.ID,
			transaction.UserID,
		)
	}

	if transaction.CreatedTimestamp.IsZero() {
		t.Error("expected created timestamp to be populated")
	}

	var storedAmountPence int64
	var storedType string
	var storedBalancePence int64

	if err := db.QueryRow(
		`
			SELECT amount_pence, transaction_type
			FROM transactions
			WHERE id = ?
		`,
		transaction.ID,
	).Scan(
		&storedAmountPence,
		&storedType,
	); err != nil {
		t.Fatalf("failed to fetch stored transaction: %v", err)
	}

	if err := db.QueryRow(
		`
			SELECT balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&storedBalancePence); err != nil {
		t.Fatalf("failed to fetch updated balance: %v", err)
	}

	if storedAmountPence != 1099 {
		t.Errorf(
			"expected stored amount of 1099 pence, got %d",
			storedAmountPence,
		)
	}

	if storedType != string(models.TransactionTypeDeposit) {
		t.Errorf(
			"expected stored type %q, got %q",
			models.TransactionTypeDeposit,
			storedType,
		)
	}

	if storedBalancePence != 1099 {
		t.Errorf(
			"expected balance of 1099 pence, got %d",
			storedBalancePence,
		)
	}
}

func createTransaction(
	t *testing.T,
	handler http.Handler,
	token string,
	accountNumber string,
	createRequest models.CreateTransactionRequest,
) models.Transaction {
	t.Helper()

	body, err := json.Marshal(createRequest)
	if err != nil {
		t.Fatalf("failed to encode transaction request: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/accounts/"+accountNumber+"/transactions",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"failed to create transaction: expected status %d, got %d; body: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	var transaction models.Transaction

	if err := json.NewDecoder(response.Body).Decode(
		&transaction,
	); err != nil {
		t.Fatalf("failed to decode transaction response: %v", err)
	}

	return transaction
}

func transactionRequest(
	amount float64,
	transactionType models.TransactionType,
) models.CreateTransactionRequest {
	return models.CreateTransactionRequest{
		Amount:   &amount,
		Currency: models.CurrencyGBP,
		Type:     transactionType,
	}
}

func TestCreateWithdrawal(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	createTransaction(
		t,
		handler,
		token,
		account.AccountNumber,
		transactionRequest(
			25,
			models.TransactionTypeDeposit,
		),
	)

	withdrawal := createTransaction(
		t,
		handler,
		token,
		account.AccountNumber,
		transactionRequest(
			10.25,
			models.TransactionTypeWithdrawal,
		),
	)

	if withdrawal.Amount != 10.25 {
		t.Errorf(
			"expected withdrawal amount 10.25, got %v",
			withdrawal.Amount,
		)
	}

	if withdrawal.Type != models.TransactionTypeWithdrawal {
		t.Errorf(
			"expected transaction type %q, got %q",
			models.TransactionTypeWithdrawal,
			withdrawal.Type,
		)
	}

	var balancePence int64

	if err := db.QueryRow(
		`
			SELECT balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&balancePence); err != nil {
		t.Fatalf("failed to fetch account balance: %v", err)
	}

	if balancePence != 1475 {
		t.Errorf(
			"expected balance of 1475 pence, got %d",
			balancePence,
		)
	}
}

func TestCreateWithdrawalRejectsInsufficientFunds(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	createTransaction(
		t,
		handler,
		token,
		account.AccountNumber,
		transactionRequest(
			5,
			models.TransactionTypeDeposit,
		),
	)

	requestBody := transactionRequest(
		10,
		models.TransactionTypeWithdrawal,
	)

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to encode withdrawal request: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/accounts/"+account.AccountNumber+"/transactions",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusUnprocessableEntity,
			response.Code,
			response.Body.String(),
		)
	}

	var balancePence int64

	if err := db.QueryRow(
		`
			SELECT balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&balancePence); err != nil {
		t.Fatalf("failed to fetch account balance: %v", err)
	}

	if balancePence != 500 {
		t.Errorf(
			"expected balance to remain 500 pence, got %d",
			balancePence,
		)
	}

	var transactionCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM transactions
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&transactionCount); err != nil {
		t.Fatalf("failed to count transactions: %v", err)
	}

	if transactionCount != 1 {
		t.Errorf(
			"expected only the deposit transaction, got %d transactions",
			transactionCount,
		)
	}
}

func TestListTransactions(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	deposit := createTransaction(
		t,
		handler,
		token,
		account.AccountNumber,
		transactionRequest(
			25,
			models.TransactionTypeDeposit,
		),
	)

	withdrawal := createTransaction(
		t,
		handler,
		token,
		account.AccountNumber,
		transactionRequest(
			10.25,
			models.TransactionTypeWithdrawal,
		),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/"+account.AccountNumber+"/transactions",
		nil,
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusOK,
			response.Code,
			response.Body.String(),
		)
	}

	var listResponse models.ListTransactionsResponse

	if err := json.NewDecoder(response.Body).Decode(
		&listResponse,
	); err != nil {
		t.Fatalf("failed to decode transaction list: %v", err)
	}

	if len(listResponse.Transactions) != 2 {
		t.Fatalf(
			"expected two transactions, got %d",
			len(listResponse.Transactions),
		)
	}

	expectedTransactions := map[string]models.Transaction{
		deposit.ID:    deposit,
		withdrawal.ID: withdrawal,
	}

	for _, transaction := range listResponse.Transactions {
		expected, exists := expectedTransactions[transaction.ID]
		if !exists {
			t.Errorf(
				"received unexpected transaction %q",
				transaction.ID,
			)
			continue
		}

		if transaction.Amount != expected.Amount {
			t.Errorf(
				"expected transaction %q amount %v, got %v",
				transaction.ID,
				expected.Amount,
				transaction.Amount,
			)
		}

		if transaction.Type != expected.Type {
			t.Errorf(
				"expected transaction %q type %q, got %q",
				transaction.ID,
				expected.Type,
				transaction.Type,
			)
		}

		delete(expectedTransactions, transaction.ID)
	}

	for transactionID := range expectedTransactions {
		t.Errorf(
			"expected transaction %q in response",
			transactionID,
		)
	}
}

func TestGetTransaction(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	createdTransaction := createTransaction(
		t,
		handler,
		token,
		account.AccountNumber,
		transactionRequest(
			25,
			models.TransactionTypeDeposit,
		),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/"+
			account.AccountNumber+
			"/transactions/"+
			createdTransaction.ID,
		nil,
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusOK,
			response.Code,
			response.Body.String(),
		)
	}

	var fetchedTransaction models.Transaction

	if err := json.NewDecoder(response.Body).Decode(
		&fetchedTransaction,
	); err != nil {
		t.Fatalf("failed to decode transaction response: %v", err)
	}

	if fetchedTransaction.ID != createdTransaction.ID {
		t.Errorf(
			"expected transaction ID %q, got %q",
			createdTransaction.ID,
			fetchedTransaction.ID,
		)
	}

	if fetchedTransaction.Amount != createdTransaction.Amount {
		t.Errorf(
			"expected amount %v, got %v",
			createdTransaction.Amount,
			fetchedTransaction.Amount,
		)
	}

	if fetchedTransaction.Type != createdTransaction.Type {
		t.Errorf(
			"expected type %q, got %q",
			createdTransaction.Type,
			fetchedTransaction.Type,
		)
	}

	if fetchedTransaction.UserID != createdTransaction.UserID {
		t.Errorf(
			"expected user ID %q, got %q",
			createdTransaction.UserID,
			fetchedTransaction.UserID,
		)
	}
}

func TestGetTransactionRejectsInvalidAccess(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, firstUserRequest := createTestUser(t, handler)

	firstUserToken := loginTestUser(
		t,
		handler,
		firstUserRequest.Email,
		firstUserRequest.Password,
	)

	firstAccount := createAccount(
		t,
		handler,
		firstUserToken,
		validCreateAccountRequest(),
	)

	secondAccountRequest := validCreateAccountRequest()
	secondAccountRequest.Name = "Second Account"

	secondAccount := createAccount(
		t,
		handler,
		firstUserToken,
		secondAccountRequest,
	)

	transaction := createTransaction(
		t,
		handler,
		firstUserToken,
		firstAccount.AccountNumber,
		transactionRequest(
			25,
			models.TransactionTypeDeposit,
		),
	)

	secondUserRequest := validCreateUserRequest()
	secondUserRequest.Name = "Another User"
	secondUserRequest.Email = "another@example.com"

	createUser(t, handler, secondUserRequest)

	secondUserToken := loginTestUser(
		t,
		handler,
		secondUserRequest.Email,
		secondUserRequest.Password,
	)

	tests := []struct {
		name               string
		token              string
		accountNumber      string
		transactionID      string
		expectedStatusCode int
	}{
		{
			name:               "another user's account",
			token:              secondUserToken,
			accountNumber:      firstAccount.AccountNumber,
			transactionID:      transaction.ID,
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:               "nonexistent account",
			token:              firstUserToken,
			accountNumber:      "01999999",
			transactionID:      transaction.ID,
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "nonexistent transaction",
			token:              firstUserToken,
			accountNumber:      firstAccount.AccountNumber,
			transactionID:      "tan-doesnotexist",
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "transaction against wrong account",
			token:              firstUserToken,
			accountNumber:      secondAccount.AccountNumber,
			transactionID:      transaction.ID,
			expectedStatusCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := fmt.Sprintf(
				"/v1/accounts/%s/transactions/%s",
				test.accountNumber,
				test.transactionID,
			)

			request := httptest.NewRequest(
				http.MethodGet,
				path,
				nil,
			)
			request.Header.Set(
				"Authorization",
				"Bearer "+test.token,
			)

			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.expectedStatusCode {
				t.Fatalf(
					"expected status %d, got %d; body: %s",
					test.expectedStatusCode,
					response.Code,
					response.Body.String(),
				)
			}

			var errorResponse models.ErrorResponse

			if err := json.NewDecoder(response.Body).Decode(
				&errorResponse,
			); err != nil {
				t.Fatalf(
					"failed to decode error response: %v",
					err,
				)
			}

			if errorResponse.Message == "" {
				t.Error("expected an error message")
			}
		})
	}
}

func TestCreateTransactionRejectsInvalidAccess(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, ownerRequest := createTestUser(t, handler)

	ownerToken := loginTestUser(
		t,
		handler,
		ownerRequest.Email,
		ownerRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		ownerToken,
		validCreateAccountRequest(),
	)

	otherUserRequest := validCreateUserRequest()
	otherUserRequest.Name = "Another User"
	otherUserRequest.Email = "another@example.com"

	createUser(t, handler, otherUserRequest)

	otherUserToken := loginTestUser(
		t,
		handler,
		otherUserRequest.Email,
		otherUserRequest.Password,
	)

	requestBody := transactionRequest(
		10,
		models.TransactionTypeDeposit,
	)

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to encode transaction request: %v", err)
	}

	nonexistentAccountNumber := "01999999"
	if account.AccountNumber == nonexistentAccountNumber {
		nonexistentAccountNumber = "01888888"
	}

	tests := []struct {
		name               string
		token              string
		accountNumber      string
		expectedStatusCode int
	}{
		{
			name:               "another user's account",
			token:              otherUserToken,
			accountNumber:      account.AccountNumber,
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:               "nonexistent account",
			token:              ownerToken,
			accountNumber:      nonexistentAccountNumber,
			expectedStatusCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := fmt.Sprintf(
				"/v1/accounts/%s/transactions",
				test.accountNumber,
			)

			request := httptest.NewRequest(
				http.MethodPost,
				path,
				bytes.NewReader(body),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(
				"Authorization",
				"Bearer "+test.token,
			)

			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.expectedStatusCode {
				t.Fatalf(
					"expected status %d, got %d; body: %s",
					test.expectedStatusCode,
					response.Code,
					response.Body.String(),
				)
			}

			var errorResponse models.ErrorResponse

			if err := json.NewDecoder(response.Body).Decode(
				&errorResponse,
			); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errorResponse.Message == "" {
				t.Error("expected an error message")
			}
		})
	}

	var balancePence int64

	if err := db.QueryRow(
		`
			SELECT balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&balancePence); err != nil {
		t.Fatalf("failed to fetch account balance: %v", err)
	}

	if balancePence != 0 {
		t.Errorf(
			"expected balance to remain 0 pence, got %d",
			balancePence,
		)
	}

	var transactionCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM transactions
		`,
	).Scan(&transactionCount); err != nil {
		t.Fatalf("failed to count transactions: %v", err)
	}

	if transactionCount != 0 {
		t.Errorf(
			"expected no transactions, got %d",
			transactionCount,
		)
	}
}

func TestListTransactionsRejectsInvalidAccess(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, ownerRequest := createTestUser(t, handler)

	ownerToken := loginTestUser(
		t,
		handler,
		ownerRequest.Email,
		ownerRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		ownerToken,
		validCreateAccountRequest(),
	)

	createTransaction(
		t,
		handler,
		ownerToken,
		account.AccountNumber,
		transactionRequest(
			25,
			models.TransactionTypeDeposit,
		),
	)

	otherUserRequest := validCreateUserRequest()
	otherUserRequest.Name = "Another User"
	otherUserRequest.Email = "another@example.com"

	createUser(t, handler, otherUserRequest)

	otherUserToken := loginTestUser(
		t,
		handler,
		otherUserRequest.Email,
		otherUserRequest.Password,
	)

	nonexistentAccountNumber := "01999999"
	if account.AccountNumber == nonexistentAccountNumber {
		nonexistentAccountNumber = "01888888"
	}

	tests := []struct {
		name               string
		token              string
		accountNumber      string
		expectedStatusCode int
	}{
		{
			name:               "another user's account",
			token:              otherUserToken,
			accountNumber:      account.AccountNumber,
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:               "nonexistent account",
			token:              ownerToken,
			accountNumber:      nonexistentAccountNumber,
			expectedStatusCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := fmt.Sprintf(
				"/v1/accounts/%s/transactions",
				test.accountNumber,
			)

			request := httptest.NewRequest(
				http.MethodGet,
				path,
				nil,
			)
			request.Header.Set(
				"Authorization",
				"Bearer "+test.token,
			)

			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.expectedStatusCode {
				t.Fatalf(
					"expected status %d, got %d; body: %s",
					test.expectedStatusCode,
					response.Code,
					response.Body.String(),
				)
			}

			var errorResponse models.ErrorResponse

			if err := json.NewDecoder(response.Body).Decode(
				&errorResponse,
			); err != nil {
				t.Fatalf(
					"failed to decode error response: %v",
					err,
				)
			}

			if errorResponse.Message == "" {
				t.Error("expected an error message")
			}
		})
	}
}

func TestCreateTransactionRejectsMissingRequiredFields(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	amount := 10.00

	tests := []struct {
		name          string
		requestBody   models.CreateTransactionRequest
		expectedField string
	}{
		{
			name: "missing amount",
			requestBody: models.CreateTransactionRequest{
				Currency: models.CurrencyGBP,
				Type:     models.TransactionTypeDeposit,
			},
			expectedField: "amount",
		},
		{
			name: "missing currency",
			requestBody: models.CreateTransactionRequest{
				Amount: &amount,
				Type:   models.TransactionTypeDeposit,
			},
			expectedField: "currency",
		},
		{
			name: "missing type",
			requestBody: models.CreateTransactionRequest{
				Amount:   &amount,
				Currency: models.CurrencyGBP,
			},
			expectedField: "type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(test.requestBody)
			if err != nil {
				t.Fatalf(
					"failed to encode transaction request: %v",
					err,
				)
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/accounts/"+
					account.AccountNumber+
					"/transactions",
				bytes.NewReader(body),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(
				"Authorization",
				"Bearer "+token,
			)

			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d; body: %s",
					http.StatusBadRequest,
					response.Code,
					response.Body.String(),
				)
			}

			var errorResponse models.BadRequestErrorResponse

			if err := json.NewDecoder(response.Body).Decode(
				&errorResponse,
			); err != nil {
				t.Fatalf(
					"failed to decode validation response: %v",
					err,
				)
			}

			foundExpectedError := false

			for _, detail := range errorResponse.Details {
				if detail.Field == test.expectedField &&
					detail.Type == "required" {
					foundExpectedError = true
					break
				}
			}

			if !foundExpectedError {
				t.Errorf(
					"expected required validation error for field %q, got %+v",
					test.expectedField,
					errorResponse.Details,
				)
			}
		})
	}

	var transactionCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM transactions
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&transactionCount); err != nil {
		t.Fatalf("failed to count transactions: %v", err)
	}

	if transactionCount != 0 {
		t.Errorf(
			"expected no transactions, got %d",
			transactionCount,
		)
	}
}

func TestCreateTransactionRejectsInvalidValues(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	invalidCurrencyRequest := transactionRequest(
		10,
		models.TransactionTypeDeposit,
	)
	invalidCurrencyRequest.Currency = models.Currency("USD")

	invalidTypeRequest := transactionRequest(
		10,
		models.TransactionType("transfer"),
	)

	tests := []struct {
		name               string
		requestBody        models.CreateTransactionRequest
		expectedField      string
		expectedDetailType string
	}{
		{
			name: "negative amount",
			requestBody: transactionRequest(
				-0.01,
				models.TransactionTypeDeposit,
			),
			expectedField:      "amount",
			expectedDetailType: "invalid",
		},
		{
			name: "amount exceeds maximum",
			requestBody: transactionRequest(
				10_000.01,
				models.TransactionTypeDeposit,
			),
			expectedField:      "amount",
			expectedDetailType: "invalid",
		},
		{
			name: "amount has more than two decimal places",
			requestBody: transactionRequest(
				10.999,
				models.TransactionTypeDeposit,
			),
			expectedField:      "amount",
			expectedDetailType: "invalid",
		},
		{
			name:               "unsupported currency",
			requestBody:        invalidCurrencyRequest,
			expectedField:      "currency",
			expectedDetailType: "enum",
		},
		{
			name:               "unsupported transaction type",
			requestBody:        invalidTypeRequest,
			expectedField:      "type",
			expectedDetailType: "enum",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(test.requestBody)
			if err != nil {
				t.Fatalf(
					"failed to encode transaction request: %v",
					err,
				)
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/accounts/"+
					account.AccountNumber+
					"/transactions",
				bytes.NewReader(body),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(
				"Authorization",
				"Bearer "+token,
			)

			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d; body: %s",
					http.StatusBadRequest,
					response.Code,
					response.Body.String(),
				)
			}

			var errorResponse models.BadRequestErrorResponse

			if err := json.NewDecoder(response.Body).Decode(
				&errorResponse,
			); err != nil {
				t.Fatalf(
					"failed to decode validation response: %v",
					err,
				)
			}

			foundExpectedError := false

			for _, detail := range errorResponse.Details {
				if detail.Field == test.expectedField &&
					detail.Type == test.expectedDetailType {
					foundExpectedError = true
					break
				}
			}

			if !foundExpectedError {
				t.Errorf(
					"expected %q validation error for field %q, got %+v",
					test.expectedDetailType,
					test.expectedField,
					errorResponse.Details,
				)
			}
		})
	}

	var balancePence int64
	var transactionCount int

	if err := db.QueryRow(
		`
			SELECT balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&balancePence); err != nil {
		t.Fatalf("failed to fetch account balance: %v", err)
	}

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM transactions
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&transactionCount); err != nil {
		t.Fatalf("failed to count transactions: %v", err)
	}

	if balancePence != 0 {
		t.Errorf(
			"expected balance to remain 0 pence, got %d",
			balancePence,
		)
	}

	if transactionCount != 0 {
		t.Errorf(
			"expected no transactions, got %d",
			transactionCount,
		)
	}
}

func TestCreateDepositRejectsMaximumBalanceExceeded(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	createTransaction(
		t,
		handler,
		token,
		account.AccountNumber,
		transactionRequest(
			10_000,
			models.TransactionTypeDeposit,
		),
	)

	requestBody := transactionRequest(
		0.01,
		models.TransactionTypeDeposit,
	)

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to encode deposit request: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/accounts/"+
			account.AccountNumber+
			"/transactions",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"Authorization",
		"Bearer "+token,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusUnprocessableEntity,
			response.Code,
			response.Body.String(),
		)
	}

	var errorResponse models.ErrorResponse

	if err := json.NewDecoder(response.Body).Decode(
		&errorResponse,
	); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errorResponse.Message == "" {
		t.Error("expected an error message")
	}

	var balancePence int64

	if err := db.QueryRow(
		`
			SELECT balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&balancePence); err != nil {
		t.Fatalf("failed to fetch account balance: %v", err)
	}

	if balancePence != 1_000_000 {
		t.Errorf(
			"expected balance to remain 1000000 pence, got %d",
			balancePence,
		)
	}

	var transactionCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM transactions
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&transactionCount); err != nil {
		t.Fatalf("failed to count transactions: %v", err)
	}

	if transactionCount != 1 {
		t.Errorf(
			"expected only the original deposit, got %d transactions",
			transactionCount,
		)
	}
}

func TestCreateTransactionRejectsMalformedJSON(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	account := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/accounts/"+
			account.AccountNumber+
			"/transactions",
		strings.NewReader(`{"amount": 10,`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"Authorization",
		"Bearer "+token,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusBadRequest,
			response.Code,
			response.Body.String(),
		)
	}

	var errorResponse models.BadRequestErrorResponse

	if err := json.NewDecoder(response.Body).Decode(
		&errorResponse,
	); err != nil {
		t.Fatalf(
			"failed to decode error response: %v",
			err,
		)
	}

	if errorResponse.Message == "" {
		t.Error("expected an error message")
	}

	if len(errorResponse.Details) != 0 {
		t.Errorf(
			"expected no field validation details, got %+v",
			errorResponse.Details,
		)
	}

	var balancePence int64
	var transactionCount int

	if err := db.QueryRow(
		`
			SELECT balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&balancePence); err != nil {
		t.Fatalf("failed to fetch account balance: %v", err)
	}

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM transactions
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&transactionCount); err != nil {
		t.Fatalf("failed to count transactions: %v", err)
	}

	if balancePence != 0 {
		t.Errorf(
			"expected balance to remain 0 pence, got %d",
			balancePence,
		)
	}

	if transactionCount != 0 {
		t.Errorf(
			"expected no transactions, got %d",
			transactionCount,
		)
	}
}
