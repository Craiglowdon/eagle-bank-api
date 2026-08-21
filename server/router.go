package server

import (
	"database/sql"
	"net/http"

	"github.com/Craiglowdon/eagle-bank-api/handlers"
	"github.com/Craiglowdon/eagle-bank-api/middleware"
)

func NewRouter(db *sql.DB, jwtSecret []byte) http.Handler {
	mux := http.NewServeMux()

	userHandler := handlers.NewUserHandler(db)
	authHandler := handlers.NewAuthHandler(db, jwtSecret)
	authenticate := middleware.Authenticate(jwtSecret)
	accountHandler := handlers.NewAccountHandler(db)
	transactionHandler := handlers.NewTransactionHandler(db)

	mux.HandleFunc("GET /health", func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /v1/users", userHandler.CreateUser)
	mux.HandleFunc("POST /v1/auth/login", authHandler.Login)

	mux.Handle(
		"GET /v1/users/{userId}",
		authenticate(http.HandlerFunc(userHandler.GetUser)),
	)

	mux.Handle(
		"PATCH /v1/users/{userId}",
		authenticate(http.HandlerFunc(userHandler.UpdateUser)),
	)

	mux.Handle(
		"DELETE /v1/users/{userId}",
		authenticate(http.HandlerFunc(userHandler.DeleteUser)),
	)

	mux.Handle(
		"POST /v1/accounts",
		authenticate(http.HandlerFunc(accountHandler.CreateAccount)),
	)

	mux.Handle(
		"GET /v1/accounts",
		authenticate(http.HandlerFunc(accountHandler.ListAccounts)),
	)

	mux.Handle(
		"GET /v1/accounts/{accountNumber}",
		authenticate(http.HandlerFunc(accountHandler.GetAccount)),
	)

	mux.Handle(
		"PATCH /v1/accounts/{accountNumber}",
		authenticate(http.HandlerFunc(accountHandler.UpdateAccount)),
	)

	mux.Handle(
		"DELETE /v1/accounts/{accountNumber}",
		authenticate(http.HandlerFunc(accountHandler.DeleteAccount)),
	)

	mux.Handle(
		"POST /v1/accounts/{accountNumber}/transactions",
		authenticate(http.HandlerFunc(transactionHandler.CreateTransaction)),
	)

	mux.Handle(
		"GET /v1/accounts/{accountNumber}/transactions",
		authenticate(http.HandlerFunc(transactionHandler.ListTransactions)),
	)

	mux.Handle(
		"GET /v1/accounts/{accountNumber}/transactions/{transactionId}",
		authenticate(http.HandlerFunc(transactionHandler.GetTransaction)),
	)

	return mux
}
