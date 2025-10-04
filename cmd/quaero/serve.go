package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server to receive auth from extension",
	Long:  `Starts the Quaero server which receives authentication from the browser extension and runs background collection.`,
	Run:   runServe,
}

func runServe(cmd *cobra.Command, args []string) {
	// Server configuration is now managed globally via config and CLI flags
	logger.Info().
		Int("port", config.Server.Port).
		Str("host", config.Server.Host).
		Msg("Starting Quaero server")

	logger.Info().Msg("Waiting for authentication from browser extension...")

	// Setup basic health endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	// Start server in goroutine
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	go func() {
		logger.Info().Str("address", addr).Msg("HTTP server starting")
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	logger.Info().Str("url", fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.Port)).Msg("Server ready")
	fmt.Printf("\nServer running on http://%s:%d\n", config.Server.Host, config.Server.Port)
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("Shutting down server...")
	fmt.Println("\nServer stopped")
}
