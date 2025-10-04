package main

import (
	"fmt"

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

	// TODO: Initialize app and start server
	logger.Warn().Msg("Server implementation pending")

	fmt.Printf("\nServer would start on http://%s:%d\n", config.Server.Host, config.Server.Port)
	fmt.Println("(Implementation pending)")
}
