package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Craiglowdon/eagle-bank-api/database"
	"github.com/Craiglowdon/eagle-bank-api/server"
)

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
		Handler: server.NewRouter(db, []byte(jwtSecret)),
	}

	log.Println("Eagle Bank API listening on :8080")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

}
