package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Craiglowdon/eagle-bank-api/models"
)

func TestCreateAccountRequiresAuthentication(t *testing.T) {
	requestBody := validCreateAccountRequest()

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to encode request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/accounts",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()

	testRoutes(t).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusUnauthorized,
			response.Code,
			response.Body.String(),
		)
	}
}

func TestCreateAccount(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	requestBody := validCreateAccountRequest()

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to encode request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/accounts",
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

	if response.Header().Get("Content-Type") != "application/json" {
		t.Errorf(
			"expected Content-Type application/json, got %q",
			response.Header().Get("Content-Type"),
		)
	}

	var account models.BankAccount

	if err := json.NewDecoder(response.Body).Decode(&account); err != nil {
		t.Fatalf("failed to decode account response: %v", err)
	}

	accountNumberPattern := regexp.MustCompile(`^01\d{6}$`)

	if !accountNumberPattern.MatchString(account.AccountNumber) {
		t.Errorf(
			"expected account number matching %q, got %q",
			accountNumberPattern,
			account.AccountNumber,
		)
	}

	if account.SortCode != "10-10-10" {
		t.Errorf(
			"expected sort code %q, got %q",
			"10-10-10",
			account.SortCode,
		)
	}

	if account.Name != requestBody.Name {
		t.Errorf(
			"expected name %q, got %q",
			requestBody.Name,
			account.Name,
		)
	}

	if account.AccountType != models.AccountTypePersonal {
		t.Errorf(
			"expected account type %q, got %q",
			models.AccountTypePersonal,
			account.AccountType,
		)
	}

	if account.Balance != 0 {
		t.Errorf("expected zero balance, got %v", account.Balance)
	}

	if account.Currency != models.CurrencyGBP {
		t.Errorf(
			"expected currency %q, got %q",
			models.CurrencyGBP,
			account.Currency,
		)
	}

	if !account.CreatedTimestamp.Equal(account.UpdatedTimestamp) {
		t.Errorf(
			"expected timestamps to match, got %s and %s",
			account.CreatedTimestamp,
			account.UpdatedTimestamp,
		)
	}

	var storedOwnerID string
	var storedBalancePence int64

	if err := db.QueryRow(
		`
			SELECT user_id, balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(
		&storedOwnerID,
		&storedBalancePence,
	); err != nil {
		t.Fatalf("failed to fetch created account: %v", err)
	}

	if storedOwnerID != createdUser.ID {
		t.Errorf(
			"expected account owner %q, got %q",
			createdUser.ID,
			storedOwnerID,
		)
	}

	if storedBalancePence != 0 {
		t.Errorf(
			"expected stored balance of zero pence, got %d",
			storedBalancePence,
		)
	}
}

func validCreateAccountRequest() models.CreateBankAccountRequest {
	return models.CreateBankAccountRequest{
		Name:        "Personal Bank Account",
		AccountType: models.AccountTypePersonal,
	}
}

func TestCreateAccountRejectsInvalidRequests(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	tests := []struct {
		name          string
		expectedField string
		modifyRequest func(*models.CreateBankAccountRequest)
	}{
		{
			name:          "missing name",
			expectedField: "name",
			modifyRequest: func(
				request *models.CreateBankAccountRequest) {
				request.Name = ""
			},
		},
		{
			name:          "missing account type",
			expectedField: "accountType",
			modifyRequest: func(
				request *models.CreateBankAccountRequest) {
				request.AccountType = ""
			},
		},
		{
			name:          "unsupported account type",
			expectedField: "accountType",
			modifyRequest: func(
				request *models.CreateBankAccountRequest) {
				request.AccountType = models.AccountType("business")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestBody := validCreateAccountRequest()
			test.modifyRequest(&requestBody)

			body, err := json.Marshal(requestBody)
			if err != nil {
				t.Fatalf("failed to encode request body: %v", err)
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/accounts",
				bytes.NewReader(body),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)

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

			if len(errorResponse.Details) != 1 {
				t.Fatalf(
					"expected one validation error, got %d",
					len(errorResponse.Details),
				)
			}

			if errorResponse.Details[0].Field != test.expectedField {
				t.Errorf(
					"expected validation error for %q, got %q",
					test.expectedField,
					errorResponse.Details[0].Field,
				)
			}
		})
	}
}

func TestListAccounts(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, firstUserRequest := createTestUser(t, handler)

	firstUserToken := loginTestUser(
		t,
		handler,
		firstUserRequest.Email,
		firstUserRequest.Password,
	)

	firstAccountRequest := validCreateAccountRequest()
	firstAccountRequest.Name = "Current Account"

	firstAccount := createAccount(
		t,
		handler,
		firstUserToken,
		firstAccountRequest,
	)

	secondAccountRequest := validCreateAccountRequest()
	secondAccountRequest.Name = "Savings Account"

	secondAccount := createAccount(
		t,
		handler,
		firstUserToken,
		secondAccountRequest,
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

	createAccount(
		t,
		handler,
		secondUserToken,
		validCreateAccountRequest(),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts",
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer "+firstUserToken,
	)

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

	var listResponse models.ListBankAccountsResponse

	if err := json.NewDecoder(response.Body).Decode(
		&listResponse,
	); err != nil {
		t.Fatalf("failed to decode account list: %v", err)
	}

	if len(listResponse.Accounts) != 2 {
		t.Fatalf(
			"expected two accounts, got %d",
			len(listResponse.Accounts),
		)
	}

	accountNumbers := map[string]bool{
		firstAccount.AccountNumber:  false,
		secondAccount.AccountNumber: false,
	}

	for _, account := range listResponse.Accounts {
		if _, expected := accountNumbers[account.AccountNumber]; !expected {
			t.Errorf(
				"received unexpected account %q",
				account.AccountNumber,
			)
			continue
		}

		accountNumbers[account.AccountNumber] = true
	}

	for accountNumber, found := range accountNumbers {
		if !found {
			t.Errorf(
				"expected account %q in response",
				accountNumber,
			)
		}
	}
}

func TestGetAccount(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	createdAccount := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/"+createdAccount.AccountNumber,
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

	var fetchedAccount models.BankAccount

	if err := json.NewDecoder(response.Body).Decode(
		&fetchedAccount,
	); err != nil {
		t.Fatalf("failed to decode account response: %v", err)
	}

	if fetchedAccount.AccountNumber != createdAccount.AccountNumber {
		t.Errorf(
			"expected account number %q, got %q",
			createdAccount.AccountNumber,
			fetchedAccount.AccountNumber,
		)
	}

	if fetchedAccount.Name != createdAccount.Name {
		t.Errorf(
			"expected name %q, got %q",
			createdAccount.Name,
			fetchedAccount.Name,
		)
	}

	if fetchedAccount.Balance != createdAccount.Balance {
		t.Errorf(
			"expected balance %v, got %v",
			createdAccount.Balance,
			fetchedAccount.Balance,
		)
	}
}

func TestGetAccountRejectsAnotherUsersAccount(t *testing.T) {
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

	requestingUserRequest := validCreateUserRequest()
	requestingUserRequest.Name = "Another User"
	requestingUserRequest.Email = "another@example.com"

	createUser(t, handler, requestingUserRequest)

	requestingUserToken := loginTestUser(
		t,
		handler,
		requestingUserRequest.Email,
		requestingUserRequest.Password,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/"+account.AccountNumber,
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer "+requestingUserToken,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusForbidden,
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
		t.Error("expected a forbidden error message")
	}
}

func TestGetAccountReturnsNotFound(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/accounts/01999999",
		nil,
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusNotFound,
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
		t.Error("expected a not-found error message")
	}
}

func TestUpdateAccountName(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	createdAccount := createAccount(
		t,
		handler,
		token,
		validCreateAccountRequest(),
	)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/v1/accounts/"+createdAccount.AccountNumber,
		bytes.NewBufferString(`{"name":"Holiday Fund"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"Authorization",
		"Bearer "+token,
	)

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

	var updatedAccount models.BankAccount

	if err := json.NewDecoder(response.Body).Decode(
		&updatedAccount,
	); err != nil {
		t.Fatalf("failed to decode updated account: %v", err)
	}

	if updatedAccount.Name != "Holiday Fund" {
		t.Errorf(
			"expected name %q, got %q",
			"Holiday Fund",
			updatedAccount.Name,
		)
	}

	if updatedAccount.AccountType != createdAccount.AccountType {
		t.Errorf(
			"expected account type to remain %q, got %q",
			createdAccount.AccountType,
			updatedAccount.AccountType,
		)
	}

	if updatedAccount.Balance != createdAccount.Balance {
		t.Errorf(
			"expected balance to remain %v, got %v",
			createdAccount.Balance,
			updatedAccount.Balance,
		)
	}

	if !updatedAccount.CreatedTimestamp.Equal(
		createdAccount.CreatedTimestamp,
	) {
		t.Errorf(
			"expected created timestamp to remain %s, got %s",
			createdAccount.CreatedTimestamp,
			updatedAccount.CreatedTimestamp,
		)
	}

	if !updatedAccount.UpdatedTimestamp.After(
		createdAccount.UpdatedTimestamp,
	) {
		t.Errorf(
			"expected updated timestamp after %s, got %s",
			createdAccount.UpdatedTimestamp,
			updatedAccount.UpdatedTimestamp,
		)
	}

	var storedName string
	var storedAccountType string
	var storedBalancePence int64

	if err := db.QueryRow(
		`
			SELECT
				name,
				account_type,
				balance_pence
			FROM accounts
			WHERE account_number = ?
		`,
		createdAccount.AccountNumber,
	).Scan(
		&storedName,
		&storedAccountType,
		&storedBalancePence,
	); err != nil {
		t.Fatalf("failed to fetch updated account: %v", err)
	}

	if storedName != "Holiday Fund" {
		t.Errorf(
			"expected stored name %q, got %q",
			"Holiday Fund",
			storedName,
		)
	}

	if storedAccountType != string(createdAccount.AccountType) {
		t.Errorf(
			"expected stored account type %q, got %q",
			createdAccount.AccountType,
			storedAccountType,
		)
	}

	if storedBalancePence != 0 {
		t.Errorf(
			"expected stored balance to remain 0 pence, got %d",
			storedBalancePence,
		)
	}
}

func TestUpdateAccountRejectsInvalidAccess(t *testing.T) {
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
			name:               "missing authentication",
			accountNumber:      account.AccountNumber,
			expectedStatusCode: http.StatusUnauthorized,
		},
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
			request := httptest.NewRequest(
				http.MethodPatch,
				"/v1/accounts/"+test.accountNumber,
				bytes.NewBufferString(`{"name":"Sneaky Update"}`),
			)
			request.Header.Set("Content-Type", "application/json")

			if test.token != "" {
				request.Header.Set(
					"Authorization",
					"Bearer "+test.token,
				)
			}

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

	var storedName string

	if err := db.QueryRow(
		`
			SELECT name
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&storedName); err != nil {
		t.Fatalf("failed to fetch stored account name: %v", err)
	}

	if storedName != account.Name {
		t.Errorf(
			"expected account name to remain %q, got %q",
			account.Name,
			storedName,
		)
	}
}

func TestUpdateAccountRejectsInvalidValues(t *testing.T) {
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

	tests := []struct {
		name               string
		requestBody        string
		expectedField      string
		expectedDetailType string
	}{
		{
			name:               "blank name",
			requestBody:        `{"name":"   "}`,
			expectedField:      "name",
			expectedDetailType: "required",
		},
		{
			name:               "empty account type",
			requestBody:        `{"accountType":""}`,
			expectedField:      "accountType",
			expectedDetailType: "required",
		},
		{
			name:               "unsupported account type",
			requestBody:        `{"accountType":"business"}`,
			expectedField:      "accountType",
			expectedDetailType: "enum",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPatch,
				"/v1/accounts/"+account.AccountNumber,
				bytes.NewBufferString(test.requestBody),
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

	var storedName string
	var storedAccountType string

	if err := db.QueryRow(
		`
			SELECT name, account_type
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(
		&storedName,
		&storedAccountType,
	); err != nil {
		t.Fatalf("failed to fetch stored account: %v", err)
	}

	if storedName != account.Name {
		t.Errorf(
			"expected account name to remain %q, got %q",
			account.Name,
			storedName,
		)
	}

	if storedAccountType != string(account.AccountType) {
		t.Errorf(
			"expected account type to remain %q, got %q",
			account.AccountType,
			storedAccountType,
		)
	}
}

func TestUpdateAccountRejectsMalformedJSON(t *testing.T) {
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
		http.MethodPatch,
		"/v1/accounts/"+account.AccountNumber,
		bytes.NewBufferString(`{"name":"Holiday Fund",`),
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

	var storedName string

	if err := db.QueryRow(
		`
			SELECT name
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&storedName); err != nil {
		t.Fatalf("failed to fetch stored account name: %v", err)
	}

	if storedName != account.Name {
		t.Errorf(
			"expected account name to remain %q, got %q",
			account.Name,
			storedName,
		)
	}
}

func TestUpdateAccountWithEmptyPatchDoesNotModifyAccount(
	t *testing.T,
) {
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

	var timestampBeforePatch string

	if err := db.QueryRow(
		`
			SELECT updated_timestamp
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&timestampBeforePatch); err != nil {
		t.Fatalf(
			"failed to fetch timestamp before patch: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		http.MethodPatch,
		"/v1/accounts/"+account.AccountNumber,
		bytes.NewBufferString(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"Authorization",
		"Bearer "+token,
	)

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

	var returnedAccount models.BankAccount

	if err := json.NewDecoder(response.Body).Decode(
		&returnedAccount,
	); err != nil {
		t.Fatalf(
			"failed to decode account response: %v",
			err,
		)
	}

	if returnedAccount.Name != account.Name {
		t.Errorf(
			"expected name to remain %q, got %q",
			account.Name,
			returnedAccount.Name,
		)
	}

	if returnedAccount.AccountType != account.AccountType {
		t.Errorf(
			"expected account type to remain %q, got %q",
			account.AccountType,
			returnedAccount.AccountType,
		)
	}

	if !returnedAccount.UpdatedTimestamp.Equal(
		account.UpdatedTimestamp,
	) {
		t.Errorf(
			"expected updated timestamp to remain %s, got %s",
			account.UpdatedTimestamp,
			returnedAccount.UpdatedTimestamp,
		)
	}

	var timestampAfterPatch string

	if err := db.QueryRow(
		`
			SELECT updated_timestamp
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&timestampAfterPatch); err != nil {
		t.Fatalf(
			"failed to fetch timestamp after patch: %v",
			err,
		)
	}

	if timestampAfterPatch != timestampBeforePatch {
		t.Errorf(
			"expected stored timestamp to remain %q, got %q",
			timestampBeforePatch,
			timestampAfterPatch,
		)
	}
}

func TestDeleteAccount(t *testing.T) {
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
		http.MethodDelete,
		"/v1/accounts/"+account.AccountNumber,
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer "+token,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusNoContent,
			response.Code,
			response.Body.String(),
		)
	}

	if response.Body.Len() != 0 {
		t.Errorf(
			"expected an empty response body, got %q",
			response.Body.String(),
		)
	}

	var accountCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&accountCount); err != nil {
		t.Fatalf("failed to count stored accounts: %v", err)
	}

	if accountCount != 0 {
		t.Errorf(
			"expected account to be deleted, found %d matching accounts",
			accountCount,
		)
	}
}

func TestDeleteAccountRejectsAccountWithTransactions(t *testing.T) {
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

	transaction := createTransaction(
		t,
		handler,
		token,
		account.AccountNumber,
		transactionRequest(
			10,
			models.TransactionTypeDeposit,
		),
	)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/v1/accounts/"+account.AccountNumber,
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer "+token,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusConflict,
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

	var accountCount int
	var balancePence int64

	if err := db.QueryRow(
		`
			SELECT COUNT(*), COALESCE(MAX(balance_pence), 0)
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(
		&accountCount,
		&balancePence,
	); err != nil {
		t.Fatalf("failed to inspect stored account: %v", err)
	}

	if accountCount != 1 {
		t.Errorf(
			"expected account to remain, found %d matching accounts",
			accountCount,
		)
	}

	if balancePence != 1000 {
		t.Errorf(
			"expected balance to remain 1000 pence, got %d",
			balancePence,
		)
	}

	var transactionCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM transactions
			WHERE id = ?
			  AND account_number = ?
		`,
		transaction.ID,
		account.AccountNumber,
	).Scan(&transactionCount); err != nil {
		t.Fatalf("failed to inspect stored transaction: %v", err)
	}

	if transactionCount != 1 {
		t.Errorf(
			"expected transaction to remain, found %d matching transactions",
			transactionCount,
		)
	}
}

func TestDeleteAccountRejectsInvalidAccess(t *testing.T) {
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
			name:               "missing authentication",
			accountNumber:      account.AccountNumber,
			expectedStatusCode: http.StatusUnauthorized,
		},
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
			request := httptest.NewRequest(
				http.MethodDelete,
				"/v1/accounts/"+test.accountNumber,
				nil,
			)

			if test.token != "" {
				request.Header.Set(
					"Authorization",
					"Bearer "+test.token,
				)
			}

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

	var accountCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM accounts
			WHERE account_number = ?
		`,
		account.AccountNumber,
	).Scan(&accountCount); err != nil {
		t.Fatalf("failed to count stored accounts: %v", err)
	}

	if accountCount != 1 {
		t.Errorf(
			"expected account to remain, found %d matching accounts",
			accountCount,
		)
	}
}
