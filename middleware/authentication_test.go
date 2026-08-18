package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte(
	"middleware-test-secret-at-least-32-characters",
)

func TestAuthenticateAcceptsValidToken(t *testing.T) {
	token := signedTestToken(
		t,
		testSecret,
		jwt.RegisteredClaims{
			Subject: "usr-test",
			Issuer:  "eagle-bank-api",
			IssuedAt: jwt.NewNumericDate(
				time.Now().Add(-time.Minute),
			),
			ExpiresAt: jwt.NewNumericDate(
				time.Now().Add(time.Hour),
			),
		},
	)

	var authenticatedUserID string

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		var ok bool

		authenticatedUserID, ok = AuthenticatedUserID(
			r.Context(),
		)
		if !ok {
			t.Error("expected authenticated user ID in context")
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := Authenticate(testSecret)(next)

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)
	request.Header.Set("Authorization", "Bearer "+token)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusNoContent,
			response.Code,
		)
	}

	if authenticatedUserID != "usr-test" {
		t.Errorf(
			"expected authenticated user ID %q, got %q",
			"usr-test",
			authenticatedUserID,
		)
	}
}

func TestAuthenticateRejectsInvalidTokens(t *testing.T) {
	now := time.Now()

	validClaims := jwt.RegisteredClaims{
		Subject:   "usr-test",
		Issuer:    "eagle-bank-api",
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	expiredClaims := jwt.RegisteredClaims{
		Subject:   "usr-test",
		Issuer:    "eagle-bank-api",
		IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
	}

	wrongIssuerClaims := jwt.RegisteredClaims{
		Subject:   "usr-test",
		Issuer:    "another-api",
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	tests := []struct {
		name                string
		authorizationHeader string
	}{
		{
			name:                "missing header",
			authorizationHeader: "",
		},
		{
			name:                "wrong authentication scheme",
			authorizationHeader: "Basic credentials",
		},
		{
			name:                "malformed token",
			authorizationHeader: "Bearer not-a-jwt",
		},
		{
			name: "incorrect signature",
			authorizationHeader: "Bearer " + signedTestToken(
				t,
				[]byte("different-secret-at-least-32-characters"),
				validClaims,
			),
		},
		{
			name: "expired token",
			authorizationHeader: "Bearer " + signedTestToken(
				t,
				testSecret,
				expiredClaims,
			),
		},
		{
			name: "incorrect issuer",
			authorizationHeader: "Bearer " + signedTestToken(
				t,
				testSecret,
				wrongIssuerClaims,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nextCalled := false

			next := http.HandlerFunc(func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				nextCalled = true
				w.WriteHeader(http.StatusNoContent)
			})

			handler := Authenticate(testSecret)(next)

			request := httptest.NewRequest(
				http.MethodGet,
				"/protected",
				nil,
			)

			if test.authorizationHeader != "" {
				request.Header.Set(
					"Authorization",
					test.authorizationHeader,
				)
			}

			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Errorf(
					"expected status %d, got %d",
					http.StatusUnauthorized,
					response.Code,
				)
			}

			if nextCalled {
				t.Error("expected middleware not to call next handler")
			}

			if response.Header().Get("Content-Type") !=
				"application/json" {
				t.Errorf(
					"expected JSON response, got %q",
					response.Header().Get("Content-Type"),
				)
			}
		})
	}
}

func signedTestToken(
	t *testing.T,
	secret []byte,
	claims jwt.RegisteredClaims,
) string {
	t.Helper()

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	signedToken, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	return signedToken
}
