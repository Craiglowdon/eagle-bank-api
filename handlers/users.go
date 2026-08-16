package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Craiglowdon/eagle-bank-api/middleware"
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

func (h *UserHandler) GetUser(
	w http.ResponseWriter,
	r *http.Request,
) {
	authenticatedUserID, ok := middleware.AuthenticatedUserID(
		r.Context(),
	)
	if !ok {
		response := models.ErrorResponse{
			Message: "access token is missing or invalid",
		}

		_ = writeJSON(w, http.StatusUnauthorized, response)
		return
	}

	requestedUserID := r.PathValue("userId")

	var user models.User
	var addressLine2 sql.NullString
	var addressLine3 sql.NullString
	var createdTimestamp string
	var updatedTimestamp string

	err := h.db.QueryRowContext(
		r.Context(),
		`
			SELECT
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
				created_timestamp,
				updated_timestamp
			FROM users
			WHERE id = ?
		`,
		requestedUserID,
	).Scan(
		&user.ID,
		&user.Name,
		&user.Address.Line1,
		&addressLine2,
		&addressLine3,
		&user.Address.Town,
		&user.Address.County,
		&user.Address.Postcode,
		&user.PhoneNumber,
		&user.Email,
		&createdTimestamp,
		&updatedTimestamp,
	)

	if errors.Is(err, sql.ErrNoRows) {
		response := models.ErrorResponse{
			Message: "user not found",
		}

		_ = writeJSON(w, http.StatusNotFound, response)
		return
	}

	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to fetch user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if authenticatedUserID != requestedUserID {
		response := models.ErrorResponse{
			Message: "you are not allowed to access this user",
		}

		_ = writeJSON(w, http.StatusForbidden, response)
		return
	}

	user.Address.Line2 = addressLine2.String
	user.Address.Line3 = addressLine3.String

	user.CreatedTimestamp, err = time.Parse(
		time.RFC3339Nano,
		createdTimestamp,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to fetch user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	user.UpdatedTimestamp, err = time.Parse(
		time.RFC3339Nano,
		updatedTimestamp,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to fetch user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	_ = writeJSON(w, http.StatusOK, user)
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
