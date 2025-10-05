package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ternarybob/quaero/internal/app"
	"github.com/ternarybob/quaero/internal/server"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server to receive auth from extension",
	Long:  `Starts the Quaero server which receives authentication from the browser extension and runs background collection.`,
	Run:   runServe,
}

func runServe(cmd *cobra.Command, args []string) {
	logger.Info().
		Int("port", config.Server.Port).
		Str("host", config.Server.Host).
		Msg("Starting Quaero server")

	// Initialize application
	application, err := app.New(config, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize application")
	}
	defer application.Close()

	// Create HTTP server
	srv := server.New(application)

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	logger.Info().Msg("Server ready")
	fmt.Printf("\nServer running on http://%s:%d\n", config.Server.Host, config.Server.Port)
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	logger.Info().Msg("Shutting down server...")
	fmt.Println("\nShutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown failed")
	}

	fmt.Println("Server stopped")
}
