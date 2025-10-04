package main

import (
	"log"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server to receive auth from extension",
	Long:  `Starts the Quaero server which receives authentication from the browser extension and runs background collection.`,
	Run:   runServe,
}

var (
	serverPort string
	serverHost string
)

func init() {
	serveCmd.Flags().StringVar(&serverPort, "port", "8080", "Server port")
	serveCmd.Flags().StringVar(&serverHost, "host", "localhost", "Server host")
}

func runServe(cmd *cobra.Command, args []string) {
	log.Printf("Quaero server starting on %s:%s", serverHost, serverPort)
	log.Println("Waiting for authentication from browser extension...")
	
	// TODO: Initialize app and start server
	log.Println("Server implementation pending")
}
