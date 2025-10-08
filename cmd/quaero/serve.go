// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 5:03:03 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

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

	// Create shutdown channel for HTTP endpoint to trigger shutdown
	shutdownChan := make(chan struct{})

	// Create HTTP server
	srv := server.New(application)
	srv.SetShutdownChannel(shutdownChan)

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	logger.Info().
		Str("url", fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.Port)).
		Msg("Server ready - Press Ctrl+C to stop")

	// Wait for interrupt signal or HTTP shutdown request
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		logger.Info().Msg("Interrupt signal received")
	case <-shutdownChan:
		logger.Info().Msg("Shutdown requested via HTTP")
	}

	// Graceful shutdown
	logger.Info().Msg("Shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown failed")
	}

	logger.Info().Msg("Server stopped")
}
