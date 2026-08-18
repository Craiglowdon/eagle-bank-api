package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Craiglowdon/eagle-bank-api/models"
	"github.com/golang-jwt/jwt/v5"
)

func TestLogin(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	createdUser, createUserRequest := createTestUser(t, handler)

	loginRequest := models.LoginRequest{
		Email:    createUserRequest.Email,
		Password: createUserRequest.Password,
	}

	loginBody, err := json.Marshal(loginRequest)
	if err != nil {
		t.Fatalf("failed to encode login request: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/login",
		bytes.NewReader(loginBody),
	)
	request.Header.Set("Content-Type", "application/json")

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

	var loginResponse models.LoginResponse

	if err := json.NewDecoder(response.Body).Decode(&loginResponse); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	if loginResponse.Token == "" {
		t.Error("expected login response to contain a token")
	}

	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(
		loginResponse.Token,
		claims,
		func(token *jwt.Token) (any, error) {
			return []byte(testJWTSecret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer("eagle-bank-api"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		t.Fatalf("failed to parse JWT: %v", err)
	}

	if !token.Valid {
		t.Fatal("expected JWT to be valid")
	}

	if claims.Subject != createdUser.ID {
		t.Errorf(
			"expected JWT subject %q, got %q",
			createdUser.ID,
			claims.Subject,
		)
	}

	if claims.IssuedAt == nil {
		t.Error("expected JWT to contain issued-at claim")
	}

	if claims.ExpiresAt == nil {
		t.Error("expected JWT to contain expiry claim")
	}

	if claims.IssuedAt != nil &&
		claims.ExpiresAt != nil &&
		!claims.ExpiresAt.After(claims.IssuedAt.Time) {
		t.Errorf(
			"expected JWT expiry %s to be after issue time %s",
			claims.ExpiresAt,
			claims.IssuedAt,
		)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	db := testDatabase(t)
	handler := NewRouter(db, []byte(testJWTSecret))

	_, createdUserRequest := createTestUser(t, handler)

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{
			name:     "incorrect password",
			email:    createdUserRequest.Email,
			password: "incorrect-password",
		},
		{
			name:     "unknown email",
			email:    "unknown@example.com",
			password: createdUserRequest.Password,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loginRequest := models.LoginRequest{
				Email:    test.email,
				Password: test.password,
			}

			body, err := json.Marshal(loginRequest)
			if err != nil {
				t.Fatalf("failed to encode login request: %v", err)
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/auth/login",
				bytes.NewReader(body),
			)
			request.Header.Set("Content-Type", "application/json")

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

			if errorResponse.Message != "invalid email or password" {
				t.Errorf(
					"expected generic credentials error, got %q",
					errorResponse.Message,
				)
			}
		})
	}
}
