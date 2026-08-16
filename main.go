package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/Craiglowdon/eagle-bank-api/database"
	"github.com/Craiglowdon/eagle-bank-api/handlers"
)

func routes(db *sql.DB, jwtSecret []byte) http.Handler {
	mux := http.NewServeMux()

	userHandler := handlers.NewUserHandler(db)
	authHandler := handlers.NewAuthHandler(db, jwtSecret)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /v1/users", userHandler.CreateUser)
	mux.HandleFunc("POST /v1/auth/login", authHandler.Login)

	return mux
}

func main() {

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must contain at least 32 characters")
	}

	db, err := database.Open("eagle-bank.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	server := &http.Server{
		Addr:    ":8080",
		Handler: routes(db, []byte(jwtSecret)),
	}

	log.Println("Eagle Bank API listening on :8080")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

}
