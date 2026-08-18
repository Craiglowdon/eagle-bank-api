package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Craiglowdon/eagle-bank-api/models"
	"golang.org/x/crypto/bcrypt"
)

func TestCreateUser(t *testing.T) {

	requestBody := validCreateUserRequest()

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to encode request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/users",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	db := testDatabase(t)

	NewRouter(db, []byte(testJWTSecret)).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	contentType := response.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf(
			"expected Content-Type application/json, got %q",
			contentType,
		)
	}

	responseBody := response.Body.Bytes()

	var user models.User

	if err := json.Unmarshal(responseBody, &user); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	var responseFields map[string]json.RawMessage

	if err := json.Unmarshal(responseBody, &responseFields); err != nil {
		t.Fatalf("failed to inspect response body: %v", err)
	}

	if _, exists := responseFields["password"]; exists {
		t.Error("response must not contain password")
	}

	if _, exists := responseFields["passwordHash"]; exists {
		t.Error("response must not contain password hash")
	}

	var storedEmail string
	var storedPasswordHash string

	if err := db.QueryRow(
		`
		SELECT email, password_hash
		FROM users
		WHERE id = ?
	`,
		user.ID,
	).Scan(&storedEmail, &storedPasswordHash); err != nil {
		t.Fatalf("failed to fetch created user from database: %v", err)
	}

	if storedEmail != requestBody.Email {
		t.Errorf(
			"expected stored email %q, got %q",
			requestBody.Email,
			storedEmail,
		)
	}

	if storedPasswordHash == requestBody.Password {
		t.Error("password was stored in plaintext")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(storedPasswordHash),
		[]byte(requestBody.Password),
	); err != nil {
		t.Errorf("stored password hash does not match password: %v", err)
	}

	if !user.CreatedTimestamp.Equal(user.UpdatedTimestamp) {
		t.Errorf(
			"expected creation and update timestamps to match, got %s and %s",
			user.CreatedTimestamp,
			user.UpdatedTimestamp,
		)
	}

	if !strings.HasPrefix(user.ID, "usr-") {
		t.Errorf("expected user ID to start with usr-, got %q", user.ID)
	}

	if user.Name != "Test User" {
		t.Errorf("expected name %q, got %q", "Test User", user.Name)
	}

	if user.Email != "test@example.com" {
		t.Errorf(
			"expected email %q, got %q",
			"test@example.com",
			user.Email,
		)
	}

	if user.CreatedTimestamp.IsZero() {
		t.Error("expected created timestamp to be populated")
	}

	if user.UpdatedTimestamp.IsZero() {
		t.Error("expected updated timestamp to be populated")
	}
}
func TestCreateUserRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name          string
		expectedField string
		removeField   func(*models.CreateUserRequest)
	}{
		{
			name:          "missing name",
			expectedField: "name",
			removeField: func(request *models.CreateUserRequest) {
				request.Name = ""
			},
		},
		{
			name:          "missing address line one",
			expectedField: "address.line1",
			removeField: func(request *models.CreateUserRequest) {
				request.Address.Line1 = ""
			},
		},
		{
			name:          "missing town",
			expectedField: "address.town",
			removeField: func(request *models.CreateUserRequest) {
				request.Address.Town = ""
			},
		},
		{
			name:          "missing county",
			expectedField: "address.county",
			removeField: func(request *models.CreateUserRequest) {
				request.Address.County = ""
			},
		},
		{
			name:          "missing postcode",
			expectedField: "address.postcode",
			removeField: func(request *models.CreateUserRequest) {
				request.Address.Postcode = ""
			},
		},
		{
			name:          "missing phone number",
			expectedField: "phoneNumber",
			removeField: func(request *models.CreateUserRequest) {
				request.PhoneNumber = ""
			},
		},
		{
			name:          "missing email",
			expectedField: "email",
			removeField: func(request *models.CreateUserRequest) {
				request.Email = ""
			},
		},
		{
			name:          "missing password",
			expectedField: "password",
			removeField: func(request *models.CreateUserRequest) {
				request.Password = ""
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestBody := validCreateUserRequest()
			test.removeField(&requestBody)

			body, err := json.Marshal(requestBody)
			if err != nil {
				t.Fatalf("failed to encode request body: %v", err)
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/users",
				bytes.NewReader(body),
			)
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()

			testRoutes(t).ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d; body: %s",
					http.StatusBadRequest,
					response.Code,
					response.Body.String(),
				)
			}

			var errorResponse models.BadRequestErrorResponse

			if err := json.NewDecoder(response.Body).Decode(&errorResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
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

func TestGetUserRequiresAuthentication(t *testing.T) {
	handler := testRoutes(t)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/users/usr-test",
		nil,
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d; body: %s",
			http.StatusUnauthorized,
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

	var errorResponse models.ErrorResponse

	if err := json.NewDecoder(response.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errorResponse.Message == "" {
		t.Error("expected an authentication error message")
	}
}

func TestGetUser(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, createRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createRequest.Email,
		createRequest.Password,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/users/"+createdUser.ID,
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

	if response.Header().Get("Content-Type") != "application/json" {
		t.Errorf(
			"expected Content-Type application/json, got %q",
			response.Header().Get("Content-Type"),
		)
	}

	var fetchedUser models.User

	if err := json.NewDecoder(response.Body).Decode(&fetchedUser); err != nil {
		t.Fatalf("failed to decode fetched user: %v", err)
	}

	if fetchedUser.ID != createdUser.ID {
		t.Errorf(
			"expected user ID %q, got %q",
			createdUser.ID,
			fetchedUser.ID,
		)
	}

	if fetchedUser.Name != createRequest.Name {
		t.Errorf(
			"expected name %q, got %q",
			createRequest.Name,
			fetchedUser.Name,
		)
	}

	if fetchedUser.Email != createRequest.Email {
		t.Errorf(
			"expected email %q, got %q",
			createRequest.Email,
			fetchedUser.Email,
		)
	}
}

func TestGetUserRejectsAnotherUser(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	targetUser, _ := createTestUser(t, handler)

	requestingUserDetails := validCreateUserRequest()
	requestingUserDetails.Name = "Another User"
	requestingUserDetails.Email = "another@example.com"

	requestingUser := createUser(
		t,
		handler,
		requestingUserDetails,
	)

	token := loginTestUser(
		t,
		handler,
		requestingUserDetails.Email,
		requestingUserDetails.Password,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/users/"+targetUser.ID,
		nil,
	)
	request.Header.Set("Authorization", "Bearer "+token)

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

	if requestingUser.ID == targetUser.ID {
		t.Fatal("test setup created the same user ID twice")
	}
}

func TestGetUserReturnsNotFound(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createRequest.Email,
		createRequest.Password,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/users/usr-doesnotexist",
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
}
