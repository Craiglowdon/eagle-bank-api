package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Craiglowdon/eagle-bank-api/database"
	"github.com/Craiglowdon/eagle-bank-api/models"
)

const testJWTSecret = "test-secret-that-is-at-least-32-characters"

func validCreateUserRequest() models.CreateUserRequest {
	return models.CreateUserRequest{
		Name: "Test User",
		Address: models.Address{
			Line1:    "1 Test Street",
			Town:     "Chester",
			County:   "Cheshire",
			Postcode: "CH1 1AA",
		},
		PhoneNumber: "+441234567890",
		Email:       "test@example.com",
		Password:    "securepassword",
	}
}

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databasePath := filepath.Join(t.TempDir(), "test.db")

	db, err := database.Open(databasePath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	})

	return db
}

func testRoutes(t *testing.T) http.Handler {
	t.Helper()

	return routes(
		testDatabase(t),
		[]byte(testJWTSecret),
	)
}

func createTestUser(
	t *testing.T,
	handler http.Handler,
) (models.User, models.CreateUserRequest) {
	t.Helper()

	createUserRequest := validCreateUserRequest()

	body, err := json.Marshal(createUserRequest)
	if err != nil {
		t.Fatalf("failed to encode create-user request: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/users",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"failed to create user: expected status %d, got %d; body: %s",
			http.StatusCreated,
			response.Code,
			response.Body.String(),
		)
	}

	var createdUser models.User

	if err := json.NewDecoder(response.Body).Decode(&createdUser); err != nil {
		t.Fatalf("failed to decode created user: %v", err)
	}

	return createdUser, createUserRequest
}
