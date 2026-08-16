package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Craiglowdon/eagle-bank-api/models"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	db *sql.DB
}

func NewUserHandler(db *sql.DB) *UserHandler {
	return &UserHandler{
		db: db,
	}
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var request models.CreateUserRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response := models.BadRequestErrorResponse{
			Message: "invalid request body",
			Details: []models.ValidationErrorDetail{},
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	if details := validateCreateUserRequest(request); len(details) > 0 {
		response := models.BadRequestErrorResponse{
			Message: "invalid request",
			Details: details,
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(request.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to create user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	now := time.Now().UTC()

	user := models.User{
		ID:               generateUserID(),
		Name:             request.Name,
		Address:          request.Address,
		PhoneNumber:      request.PhoneNumber,
		Email:            request.Email,
		CreatedTimestamp: now,
		UpdatedTimestamp: now,
	}

	_, err = h.db.ExecContext(
		r.Context(),
		`
		INSERT INTO users (
			id,
			name,
			address_line1,
			address_line2,
			address_line3,
			town,
			county,
			postcode,
			phone_number,
			email,
			password_hash,
			created_timestamp,
			updated_timestamp
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		user.ID,
		user.Name,
		user.Address.Line1,
		user.Address.Line2,
		user.Address.Line3,
		user.Address.Town,
		user.Address.County,
		user.Address.Postcode,
		user.PhoneNumber,
		user.Email,
		string(passwordHash),
		user.CreatedTimestamp.Format(time.RFC3339Nano),
		user.UpdatedTimestamp.Format(time.RFC3339Nano),
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to create user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	_ = writeJSON(w, http.StatusCreated, user)

}

func generateUserID() string {
	bytes := make([]byte, 8)

	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}

	return "usr-" + hex.EncodeToString(bytes)
}

func validateCreateUserRequest(
	request models.CreateUserRequest,
) []models.ValidationErrorDetail {
	var details []models.ValidationErrorDetail

	addRequiredError := func(field string) {
		details = append(details, models.ValidationErrorDetail{
			Field:   field,
			Message: field + " is required",
			Type:    "required",
		})
	}

	if strings.TrimSpace(request.Name) == "" {
		addRequiredError("name")
	}

	if strings.TrimSpace(request.Address.Line1) == "" {
		addRequiredError("address.line1")
	}

	if strings.TrimSpace(request.Address.Town) == "" {
		addRequiredError("address.town")
	}

	if strings.TrimSpace(request.Address.County) == "" {
		addRequiredError("address.county")
	}

	if strings.TrimSpace(request.Address.Postcode) == "" {
		addRequiredError("address.postcode")
	}

	if strings.TrimSpace(request.PhoneNumber) == "" {
		addRequiredError("phoneNumber")
	}

	if strings.TrimSpace(request.Email) == "" {
		addRequiredError("email")
	}

	if strings.TrimSpace(request.Password) == "" {
		addRequiredError("password")
	}

	return details
}
