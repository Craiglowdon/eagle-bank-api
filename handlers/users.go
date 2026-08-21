package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/Craiglowdon/eagle-bank-api/middleware"
	"github.com/Craiglowdon/eagle-bank-api/models"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	db *sql.DB
}

var phoneNumberPattern = regexp.MustCompile(
	`^\+[1-9][0-9]{1,14}$`,
)

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

	user, err := h.fetchUser(
		r.Context(),
		requestedUserID,
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

	_ = writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) UpdateUser(
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

	var request models.UpdateUserRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response := models.BadRequestErrorResponse{
			Message: "invalid request body",
			Details: []models.ValidationErrorDetail{},
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	requestedUserID := r.PathValue("userId")

	existingUser, err := h.fetchUser(
		r.Context(),
		requestedUserID,
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
			Message: "failed to update user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if authenticatedUserID != requestedUserID {
		response := models.ErrorResponse{
			Message: "you are not allowed to update this user",
		}

		_ = writeJSON(w, http.StatusForbidden, response)
		return
	}

	if request.Name == nil &&
		request.Address == nil &&
		request.PhoneNumber == nil &&
		request.Email == nil {
		_ = writeJSON(
			w,
			http.StatusOK,
			existingUser,
		)
		return
	}

	updatedUser := existingUser

	if request.Name != nil {
		updatedUser.Name = *request.Name
	}

	if request.Address != nil {
		updatedUser.Address = *request.Address
	}

	if request.PhoneNumber != nil {
		updatedUser.PhoneNumber = *request.PhoneNumber
	}

	if request.Email != nil {
		updatedUser.Email = *request.Email
	}

	if details := validateUserDetails(
		updatedUser.Name,
		updatedUser.Address,
		updatedUser.PhoneNumber,
		updatedUser.Email,
	); len(details) > 0 {
		response := models.BadRequestErrorResponse{
			Message: "invalid request",
			Details: details,
		}

		_ = writeJSON(w, http.StatusBadRequest, response)
		return
	}

	if request.Email != nil {
		var existingUserID string

		err = h.db.QueryRowContext(
			r.Context(),
			`
			SELECT id
			FROM users
			WHERE email = ? COLLATE NOCASE
			  AND id <> ?
		`,
			updatedUser.Email,
			requestedUserID,
		).Scan(&existingUserID)

		if err == nil {
			response := models.BadRequestErrorResponse{
				Message: "invalid request",
				Details: []models.ValidationErrorDetail{
					{
						Field:   "email",
						Message: "email cannot be used",
						Type:    "invalid",
					},
				},
			}

			_ = writeJSON(
				w,
				http.StatusBadRequest,
				response,
			)
			return
		}

		if !errors.Is(err, sql.ErrNoRows) {
			response := models.ErrorResponse{
				Message: "failed to update user",
			}

			_ = writeJSON(
				w,
				http.StatusInternalServerError,
				response,
			)
			return
		}
	}

	updatedUser.UpdatedTimestamp = time.Now().UTC()

	_, err = h.db.ExecContext(
		r.Context(),
		`
			UPDATE users
			SET
				name = ?,
				address_line1 = ?,
				address_line2 = ?,
				address_line3 = ?,
				town = ?,
				county = ?,
				postcode = ?,
				phone_number = ?,
				email = ?,
				updated_timestamp = ?
			WHERE id = ?
		`,
		updatedUser.Name,
		updatedUser.Address.Line1,
		updatedUser.Address.Line2,
		updatedUser.Address.Line3,
		updatedUser.Address.Town,
		updatedUser.Address.County,
		updatedUser.Address.Postcode,
		updatedUser.PhoneNumber,
		updatedUser.Email,
		updatedUser.UpdatedTimestamp.Format(time.RFC3339Nano),
		requestedUserID,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to update user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	_ = writeJSON(w, http.StatusOK, updatedUser)
}

func (h *UserHandler) DeleteUser(
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

	databaseTransaction, err := h.db.BeginTx(
		r.Context(),
		nil,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to delete user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}
	defer databaseTransaction.Rollback()

	var storedUserID string

	err = databaseTransaction.QueryRowContext(
		r.Context(),
		`
			SELECT id
			FROM users
			WHERE id = ?
		`,
		requestedUserID,
	).Scan(&storedUserID)

	if errors.Is(err, sql.ErrNoRows) {
		response := models.ErrorResponse{
			Message: "user not found",
		}

		_ = writeJSON(w, http.StatusNotFound, response)
		return
	}

	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to delete user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if storedUserID != authenticatedUserID {
		response := models.ErrorResponse{
			Message: "you are not allowed to delete this user",
		}

		_ = writeJSON(w, http.StatusForbidden, response)
		return
	}

	var accountCount int

	err = databaseTransaction.QueryRowContext(
		r.Context(),
		`
		SELECT COUNT(*)
		FROM accounts
		WHERE user_id = ?
	`,
		requestedUserID,
	).Scan(&accountCount)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to delete user",
		}

		_ = writeJSON(
			w,
			http.StatusInternalServerError,
			response,
		)
		return
	}

	if accountCount > 0 {
		response := models.ErrorResponse{
			Message: "user cannot be deleted while they have accounts",
		}

		_ = writeJSON(
			w,
			http.StatusConflict,
			response,
		)
		return
	}

	_, err = databaseTransaction.ExecContext(
		r.Context(),
		`
			DELETE FROM users
			WHERE id = ?
		`,
		requestedUserID,
	)
	if err != nil {
		response := models.ErrorResponse{
			Message: "failed to delete user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	if err := databaseTransaction.Commit(); err != nil {
		response := models.ErrorResponse{
			Message: "failed to delete user",
		}

		_ = writeJSON(w, http.StatusInternalServerError, response)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func generateUserID() string {
	bytes := make([]byte, 8)

	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}

	return "usr-" + hex.EncodeToString(bytes)
}

func validateUserDetails(
	name string,
	address models.Address,
	phoneNumber string,
	email string,
) []models.ValidationErrorDetail {
	var details []models.ValidationErrorDetail

	addRequiredError := func(field string) {
		details = append(
			details,
			models.ValidationErrorDetail{
				Field:   field,
				Message: field + " is required",
				Type:    "required",
			},
		)
	}

	addFormatError := func(field string) {
		details = append(
			details,
			models.ValidationErrorDetail{
				Field:   field,
				Message: field + " has an invalid format",
				Type:    "format",
			},
		)
	}

	if strings.TrimSpace(name) == "" {
		addRequiredError("name")
	}

	if strings.TrimSpace(address.Line1) == "" {
		addRequiredError("address.line1")
	}

	if strings.TrimSpace(address.Town) == "" {
		addRequiredError("address.town")
	}

	if strings.TrimSpace(address.County) == "" {
		addRequiredError("address.county")
	}

	if strings.TrimSpace(address.Postcode) == "" {
		addRequiredError("address.postcode")
	}

	if strings.TrimSpace(phoneNumber) == "" {
		addRequiredError("phoneNumber")
	} else if !phoneNumberPattern.MatchString(phoneNumber) {
		addFormatError("phoneNumber")
	}

	if strings.TrimSpace(email) == "" {
		addRequiredError("email")
	} else {
		parsedAddress, err := mail.ParseAddress(email)

		if err != nil || parsedAddress.Address != email {
			addFormatError("email")
		}
	}

	return details
}

func validateCreateUserRequest(
	request models.CreateUserRequest,
) []models.ValidationErrorDetail {
	details := validateUserDetails(
		request.Name,
		request.Address,
		request.PhoneNumber,
		request.Email,
	)

	if strings.TrimSpace(request.Password) == "" {
		details = append(
			details,
			models.ValidationErrorDetail{
				Field:   "password",
				Message: "password is required",
				Type:    "required",
			},
		)
	}

	return details
}

func (h *UserHandler) fetchUser(
	ctx context.Context,
	userID string,
) (models.User, error) {
	var user models.User
	var addressLine2 sql.NullString
	var addressLine3 sql.NullString
	var createdTimestamp string
	var updatedTimestamp string

	err := h.db.QueryRowContext(
		ctx,
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
		userID,
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
	if err != nil {
		return models.User{}, fmt.Errorf("scan user: %w", err)
	}

	user.Address.Line2 = addressLine2.String
	user.Address.Line3 = addressLine3.String

	user.CreatedTimestamp, err = time.Parse(
		time.RFC3339Nano,
		createdTimestamp,
	)
	if err != nil {
		return models.User{},
			fmt.Errorf("parse created timestamp: %w", err)
	}

	user.UpdatedTimestamp, err = time.Parse(
		time.RFC3339Nano,
		updatedTimestamp,
	)
	if err != nil {
		return models.User{},
			fmt.Errorf("parse updated timestamp: %w", err)
	}

	return user, nil
}
