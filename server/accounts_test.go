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
