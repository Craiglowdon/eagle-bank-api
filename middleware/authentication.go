package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Craiglowdon/eagle-bank-api/models"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const authenticatedUserIDKey contextKey = "authenticatedUserID"

func Authenticate(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			authorization := r.Header.Get("Authorization")
			parts := strings.Fields(authorization)

			if len(parts) != 2 ||
				!strings.EqualFold(parts[0], "Bearer") {
				writeUnauthorized(w)
				return
			}

			claims := &jwt.RegisteredClaims{}

			token, err := jwt.ParseWithClaims(
				parts[1],
				claims,
				func(token *jwt.Token) (any, error) {
					return jwtSecret, nil
				},
				jwt.WithValidMethods(
					[]string{jwt.SigningMethodHS256.Alg()},
				),
				jwt.WithIssuer("eagle-bank-api"),
				jwt.WithExpirationRequired(),
			)
			if err != nil ||
				!token.Valid ||
				claims.Subject == "" {
				writeUnauthorized(w)
				return
			}

			ctx := context.WithValue(
				r.Context(),
				authenticatedUserIDKey,
				claims.Subject,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AuthenticatedUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(authenticatedUserIDKey).(string)
	return userID, ok
}

func writeUnauthorized(w http.ResponseWriter) {
	response := models.ErrorResponse{
		Message: "access token is missing or invalid",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	_ = json.NewEncoder(w).Encode(response)
}
