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

func TestUpdateUserName(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	var passwordHashBeforePatch string

	if err := db.QueryRow(
		`
			SELECT password_hash
			FROM users
			WHERE id = ?
		`,
		createdUser.ID,
	).Scan(&passwordHashBeforePatch); err != nil {
		t.Fatalf(
			"failed to fetch password hash before patch: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		http.MethodPatch,
		"/v1/users/"+createdUser.ID,
		bytes.NewBufferString(`{"name":"Updated User"}`),
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

	var updatedUser models.User

	if err := json.NewDecoder(response.Body).Decode(
		&updatedUser,
	); err != nil {
		t.Fatalf("failed to decode updated user: %v", err)
	}

	if updatedUser.Name != "Updated User" {
		t.Errorf(
			"expected name %q, got %q",
			"Updated User",
			updatedUser.Name,
		)
	}

	if updatedUser.Email != createdUser.Email {
		t.Errorf(
			"expected email to remain %q, got %q",
			createdUser.Email,
			updatedUser.Email,
		)
	}

	if updatedUser.PhoneNumber != createdUser.PhoneNumber {
		t.Errorf(
			"expected phone number to remain %q, got %q",
			createdUser.PhoneNumber,
			updatedUser.PhoneNumber,
		)
	}

	if updatedUser.Address != createdUser.Address {
		t.Errorf(
			"expected address to remain %+v, got %+v",
			createdUser.Address,
			updatedUser.Address,
		)
	}

	if !updatedUser.CreatedTimestamp.Equal(
		createdUser.CreatedTimestamp,
	) {
		t.Errorf(
			"expected created timestamp to remain %s, got %s",
			createdUser.CreatedTimestamp,
			updatedUser.CreatedTimestamp,
		)
	}

	if !updatedUser.UpdatedTimestamp.After(
		createdUser.UpdatedTimestamp,
	) {
		t.Errorf(
			"expected updated timestamp after %s, got %s",
			createdUser.UpdatedTimestamp,
			updatedUser.UpdatedTimestamp,
		)
	}

	var storedName string
	var passwordHashAfterPatch string

	if err := db.QueryRow(
		`
			SELECT name, password_hash
			FROM users
			WHERE id = ?
		`,
		createdUser.ID,
	).Scan(
		&storedName,
		&passwordHashAfterPatch,
	); err != nil {
		t.Fatalf("failed to fetch updated user: %v", err)
	}

	if storedName != "Updated User" {
		t.Errorf(
			"expected stored name %q, got %q",
			"Updated User",
			storedName,
		)
	}

	if passwordHashAfterPatch != passwordHashBeforePatch {
		t.Error("expected password hash to remain unchanged")
	}
}

func TestUpdateUserRejectsInvalidAccess(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, ownerRequest := createTestUser(t, handler)

	ownerToken := loginTestUser(
		t,
		handler,
		ownerRequest.Email,
		ownerRequest.Password,
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

	tests := []struct {
		name               string
		token              string
		userID             string
		expectedStatusCode int
	}{
		{
			name:               "missing authentication",
			userID:             createdUser.ID,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "another user",
			token:              otherUserToken,
			userID:             createdUser.ID,
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:               "nonexistent user",
			token:              ownerToken,
			userID:             "usr-notfound",
			expectedStatusCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPatch,
				"/v1/users/"+test.userID,
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
			FROM users
			WHERE id = ?
		`,
		createdUser.ID,
	).Scan(&storedName); err != nil {
		t.Fatalf("failed to fetch stored user: %v", err)
	}

	if storedName != createdUser.Name {
		t.Errorf(
			"expected user name to remain %q, got %q",
			createdUser.Name,
			storedName,
		)
	}
}

func TestUpdateUserRejectsInvalidValues(t *testing.T) {
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
			name: "incomplete address",
			requestBody: `{
				"address": {
					"line1": "2 Updated Street"
				}
			}`,
			expectedField:      "address.town",
			expectedDetailType: "required",
		},
		{
			name:               "blank phone number",
			requestBody:        `{"phoneNumber":"   "}`,
			expectedField:      "phoneNumber",
			expectedDetailType: "required",
		},
		{
			name:               "blank email",
			requestBody:        `{"email":"   "}`,
			expectedField:      "email",
			expectedDetailType: "required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := testDatabase(t)
			handler := NewRouter(db, []byte(testJWTSecret))

			createdUser, createUserRequest := createTestUser(
				t,
				handler,
			)

			token := loginTestUser(
				t,
				handler,
				createUserRequest.Email,
				createUserRequest.Password,
			)

			request := httptest.NewRequest(
				http.MethodPatch,
				"/v1/users/"+createdUser.ID,
				bytes.NewBufferString(test.requestBody),
			)
			request.Header.Set(
				"Content-Type",
				"application/json",
			)
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
					"expected %q validation error for %q, got %+v",
					test.expectedDetailType,
					test.expectedField,
					errorResponse.Details,
				)
			}

			var storedName string
			var storedTown string
			var storedPhoneNumber string
			var storedEmail string

			if err := db.QueryRow(
				`
					SELECT
						name,
						town,
						phone_number,
						email
					FROM users
					WHERE id = ?
				`,
				createdUser.ID,
			).Scan(
				&storedName,
				&storedTown,
				&storedPhoneNumber,
				&storedEmail,
			); err != nil {
				t.Fatalf(
					"failed to fetch stored user: %v",
					err,
				)
			}

			if storedName != createdUser.Name {
				t.Errorf(
					"expected name to remain %q, got %q",
					createdUser.Name,
					storedName,
				)
			}

			if storedTown != createdUser.Address.Town {
				t.Errorf(
					"expected town to remain %q, got %q",
					createdUser.Address.Town,
					storedTown,
				)
			}

			if storedPhoneNumber != createdUser.PhoneNumber {
				t.Errorf(
					"expected phone number to remain %q, got %q",
					createdUser.PhoneNumber,
					storedPhoneNumber,
				)
			}

			if storedEmail != createdUser.Email {
				t.Errorf(
					"expected email to remain %q, got %q",
					createdUser.Email,
					storedEmail,
				)
			}
		})
	}
}

func TestUpdateUserWithEmptyPatchDoesNotModifyUser(
	t *testing.T,
) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	var timestampBeforePatch string

	if err := db.QueryRow(
		`
			SELECT updated_timestamp
			FROM users
			WHERE id = ?
		`,
		createdUser.ID,
	).Scan(&timestampBeforePatch); err != nil {
		t.Fatalf(
			"failed to fetch timestamp before patch: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		http.MethodPatch,
		"/v1/users/"+createdUser.ID,
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

	var returnedUser models.User

	if err := json.NewDecoder(response.Body).Decode(
		&returnedUser,
	); err != nil {
		t.Fatalf("failed to decode returned user: %v", err)
	}

	if returnedUser.Name != createdUser.Name {
		t.Errorf(
			"expected name to remain %q, got %q",
			createdUser.Name,
			returnedUser.Name,
		)
	}

	if returnedUser.Address != createdUser.Address {
		t.Errorf(
			"expected address to remain %+v, got %+v",
			createdUser.Address,
			returnedUser.Address,
		)
	}

	if returnedUser.PhoneNumber != createdUser.PhoneNumber {
		t.Errorf(
			"expected phone number to remain %q, got %q",
			createdUser.PhoneNumber,
			returnedUser.PhoneNumber,
		)
	}

	if returnedUser.Email != createdUser.Email {
		t.Errorf(
			"expected email to remain %q, got %q",
			createdUser.Email,
			returnedUser.Email,
		)
	}

	if !returnedUser.UpdatedTimestamp.Equal(
		createdUser.UpdatedTimestamp,
	) {
		t.Errorf(
			"expected updated timestamp to remain %s, got %s",
			createdUser.UpdatedTimestamp,
			returnedUser.UpdatedTimestamp,
		)
	}

	var timestampAfterPatch string

	if err := db.QueryRow(
		`
			SELECT updated_timestamp
			FROM users
			WHERE id = ?
		`,
		createdUser.ID,
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

func TestUpdateUserRejectsMalformedJSON(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, createUserRequest := createTestUser(t, handler)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/v1/users/"+createdUser.ID,
		bytes.NewBufferString(`{"name":"Updated User",`),
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
			FROM users
			WHERE id = ?
		`,
		createdUser.ID,
	).Scan(&storedName); err != nil {
		t.Fatalf("failed to fetch stored user: %v", err)
	}

	if storedName != createdUser.Name {
		t.Errorf(
			"expected name to remain %q, got %q",
			createdUser.Name,
			storedName,
		)
	}
}

func TestCreateUserRejectsInvalidFormats(t *testing.T) {
	tests := []struct {
		name          string
		expectedField string
		modifyRequest func(*models.CreateUserRequest)
	}{
		{
			name:          "invalid phone number",
			expectedField: "phoneNumber",
			modifyRequest: func(
				request *models.CreateUserRequest,
			) {
				request.PhoneNumber = "01234567890"
			},
		},
		{
			name:          "invalid email",
			expectedField: "email",
			modifyRequest: func(
				request *models.CreateUserRequest,
			) {
				request.Email = "not-an-email"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := testDatabase(t)
			handler := NewRouter(
				db,
				[]byte(testJWTSecret),
			)

			requestBody := validCreateUserRequest()
			test.modifyRequest(&requestBody)

			body, err := json.Marshal(requestBody)
			if err != nil {
				t.Fatalf(
					"failed to encode request body: %v",
					err,
				)
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/users",
				bytes.NewReader(body),
			)
			request.Header.Set(
				"Content-Type",
				"application/json",
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
					detail.Type == "format" {
					foundExpectedError = true
					break
				}
			}

			if !foundExpectedError {
				t.Errorf(
					"expected format error for %q, got %+v",
					test.expectedField,
					errorResponse.Details,
				)
			}

			var userCount int

			if err := db.QueryRow(
				`SELECT COUNT(*) FROM users`,
			).Scan(&userCount); err != nil {
				t.Fatalf(
					"failed to count stored users: %v",
					err,
				)
			}

			if userCount != 0 {
				t.Errorf(
					"expected no users to be created, got %d",
					userCount,
				)
			}
		})
	}
}

func TestUpdateUserRejectsUnavailableEmail(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	firstUser, firstUserRequest := createTestUser(
		t,
		handler,
	)

	firstUserToken := loginTestUser(
		t,
		handler,
		firstUserRequest.Email,
		firstUserRequest.Password,
	)

	secondUserRequest := validCreateUserRequest()
	secondUserRequest.Name = "Another User"
	secondUserRequest.Email = "another@example.com"

	secondUser := createUser(
		t,
		handler,
		secondUserRequest,
	)

	requestBody := models.UpdateUserRequest{
		Email: &secondUserRequest.Email,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf(
			"failed to encode update request: %v",
			err,
		)
	}

	request := httptest.NewRequest(
		http.MethodPatch,
		"/v1/users/"+firstUser.ID,
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(
		"Authorization",
		"Bearer "+firstUserToken,
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

	foundEmailError := false

	for _, detail := range errorResponse.Details {
		if detail.Field == "email" &&
			detail.Type == "invalid" &&
			detail.Message == "email cannot be used" {
			foundEmailError = true
			break
		}
	}

	if !foundEmailError {
		t.Errorf(
			"expected neutral email validation error, got %+v",
			errorResponse.Details,
		)
	}

	var storedFirstEmail string

	if err := db.QueryRow(
		`
			SELECT email
			FROM users
			WHERE id = ?
		`,
		firstUser.ID,
	).Scan(&storedFirstEmail); err != nil {
		t.Fatalf(
			"failed to fetch first user email: %v",
			err,
		)
	}

	if storedFirstEmail != firstUser.Email {
		t.Errorf(
			"expected first email to remain %q, got %q",
			firstUser.Email,
			storedFirstEmail,
		)
	}

	var storedSecondEmail string

	if err := db.QueryRow(
		`
			SELECT email
			FROM users
			WHERE id = ?
		`,
		secondUser.ID,
	).Scan(&storedSecondEmail); err != nil {
		t.Fatalf(
			"failed to fetch second user email: %v",
			err,
		)
	}

	if storedSecondEmail != secondUser.Email {
		t.Errorf(
			"expected second email to remain %q, got %q",
			secondUser.Email,
			storedSecondEmail,
		)
	}
}

func TestDeleteUser(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, createUserRequest := createTestUser(
		t,
		handler,
	)

	token := loginTestUser(
		t,
		handler,
		createUserRequest.Email,
		createUserRequest.Password,
	)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/v1/users/"+createdUser.ID,
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

	var userCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM users
			WHERE id = ?
		`,
		createdUser.ID,
	).Scan(&userCount); err != nil {
		t.Fatalf("failed to count stored users: %v", err)
	}

	if userCount != 0 {
		t.Errorf(
			"expected user to be deleted, found %d matching users",
			userCount,
		)
	}
}

func TestDeleteUserRejectsUserWithAccount(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, createUserRequest := createTestUser(
		t,
		handler,
	)

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
		"/v1/users/"+createdUser.ID,
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

	var userCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM users
			WHERE id = ?
		`,
		createdUser.ID,
	).Scan(&userCount); err != nil {
		t.Fatalf("failed to count stored users: %v", err)
	}

	if userCount != 1 {
		t.Errorf(
			"expected user to remain, found %d matching users",
			userCount,
		)
	}

	var accountCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM accounts
			WHERE account_number = ?
			  AND user_id = ?
		`,
		account.AccountNumber,
		createdUser.ID,
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

func TestDeleteUserRejectsInvalidAccess(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, ownerRequest := createTestUser(
		t,
		handler,
	)

	ownerToken := loginTestUser(
		t,
		handler,
		ownerRequest.Email,
		ownerRequest.Password,
	)

	otherUserRequest := validCreateUserRequest()
	otherUserRequest.Name = "Another User"
	otherUserRequest.Email = "another@example.com"

	otherUser := createUser(
		t,
		handler,
		otherUserRequest,
	)

	otherUserToken := loginTestUser(
		t,
		handler,
		otherUserRequest.Email,
		otherUserRequest.Password,
	)

	tests := []struct {
		name               string
		token              string
		userID             string
		expectedStatusCode int
	}{
		{
			name:               "missing authentication",
			userID:             createdUser.ID,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "another user",
			token:              otherUserToken,
			userID:             createdUser.ID,
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:               "nonexistent user",
			token:              ownerToken,
			userID:             "usr-notfound",
			expectedStatusCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodDelete,
				"/v1/users/"+test.userID,
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

	var firstUserCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM users
			WHERE id = ?
		`,
		createdUser.ID,
	).Scan(&firstUserCount); err != nil {
		t.Fatalf("failed to count first user: %v", err)
	}

	if firstUserCount != 1 {
		t.Errorf(
			"expected first user to remain, found %d matching users",
			firstUserCount,
		)
	}

	var secondUserCount int

	if err := db.QueryRow(
		`
			SELECT COUNT(*)
			FROM users
			WHERE id = ?
		`,
		otherUser.ID,
	).Scan(&secondUserCount); err != nil {
		t.Fatalf("failed to count second user: %v", err)
	}

	if secondUserCount != 1 {
		t.Errorf(
			"expected second user to remain, found %d matching users",
			secondUserCount,
		)
	}
}
