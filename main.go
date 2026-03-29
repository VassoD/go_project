package main

import (
	"log"
	"net/http"
	"os"
	_ "time/tzdata"
	"vtc-service/internal/handler"
	"vtc-service/internal/store"
)

func main() {
	// Create the shared store — one instance, shared by all handlers.
	s := store.New()

	// Create a ServeMux, Go's built-in HTTP router.
	mux := http.NewServeMux()

	// Register routes
	mux.Handle("POST /ingest", &handler.IngestHandler{Store: s})
	mux.Handle("GET /balances", &handler.BalanceHandler{Store: s})

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Read port from environment variable, default to 8080.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("VTC service starting on :%s", port)

	// http.ListenAndServe blocks until the server exits.
	// log.Fatal prints the error and calls os.Exit(1)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
