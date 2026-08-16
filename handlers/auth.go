package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Craiglowdon/eagle-bank-api/models"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db        *sql.DB
	jwtSecret []byte
}

func NewAuthHandler(db *sql.DB, jwtSecret []byte) *AuthHandler {
	return &AuthHandler{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var request models.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response := models.BadRequestErrorResponse{
			Message: "invalid request body",
			Details: []models.ValidationErrorDetail{},
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	if strings.TrimSpace(request.Email) == "" ||
		strings.TrimSpace(request.Password) == "" {
		response := models.ErrorResponse{
			Message: "email and password are required",
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	var userID string
	var passwordHash string

	err := h.db.QueryRowContext(
		r.Context(),
		`
			SELECT id, password_hash
			FROM users
			WHERE email = ? COLLATE NOCASE
		`,
		request.Email,
	).Scan(&userID, &passwordHash)

	if errors.Is(err, sql.ErrNoRows) {
		h.writeInvalidCredentials(w)
		return
	}

	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to authenticate user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(request.Password),
	); err != nil {
		h.writeInvalidCredentials(w)
		return
	}

	now := time.Now().UTC()

	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    "eagle-bank-api",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(h.jwtSecret)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to authenticate user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	response := models.LoginResponse{
		Token: signedToken,
	}

	_ = writeJSON(w, http.StatusOK, response)
}

func (h *AuthHandler) writeInvalidCredentials(w http.ResponseWriter) {
	response := models.ErrorResponse{
		Message: "invalid email or password",
	}

	_ = writeJSON(w, http.StatusUnauthorized, response)
}
