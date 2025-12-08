package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/example/multi-lang-test/pkg"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Initialize logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Create router
	r := mux.NewRouter()

	// Register routes
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/api/health", healthHandler).Methods("GET")
	r.HandleFunc("/api/process", processHandler).Methods("POST")

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	addr := fmt.Sprintf(":%s", port)
	log.Info().Str("addr", addr).Msg("Starting server")

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<h1>Multi-Language Test Project</h1>")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","version":"1.0.0"}`)
}

func processHandler(w http.ResponseWriter, r *http.Request) {
	// Use utility functions
	result := pkg.ProcessData("sample data")

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"result":"%s"}`, result)
}
