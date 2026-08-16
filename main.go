package main

import (
	"log"
	"net/http"
)

func routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func main() {
	server := &http.Server{
		Addr:    ":8080",
		Handler: routes(),
	}

	log.Println("Eagle Bank API listening on :8080")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

}
