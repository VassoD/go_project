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
	s := store.New()

	mux := http.NewServeMux()
	mux.Handle("POST /ingest", &handler.IngestHandler{Store: s})
	mux.Handle("GET /balances", &handler.BalanceHandler{Store: s})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("VTC service starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
