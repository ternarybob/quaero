package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

var (
	// Global flags
	configPath string
	serverPort int
	serverHost string

	// Global state
	config *common.Config
	logger arbor.ILogger
)

var rootCmd = &cobra.Command{
	Use:   "quaero",
	Short: "Quaero - Knowledge search system",
	Long:  `Quaero (Latin: "I seek") - A local knowledge base system with natural language queries.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Startup sequence: config load -> logging -> banner -> information
		var err error

		// 1. Load configuration (default -> file -> env -> CLI)
		config, err = common.LoadFromFile(configPath)
		if err != nil {
			// Use temporary logger for startup errors
			tempLogger := arbor.NewLogger()
			tempLogger.Fatal().Str("path", configPath).Err(err).Msg("Failed to load configuration")
			os.Exit(1)
		}

		// Apply CLI flag overrides (highest priority)
		common.ApplyCLIOverrides(config, serverPort, serverHost)

		// 2. Initialize logger with final configuration
		logger = common.InitLogger(config)

		// 3. Print banner with configuration and logger
		common.PrintBanner(config, logger)
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		if logger != nil {
			logger.Fatal().Err(err).Msg("Command execution failed")
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().IntVarP(&serverPort, "port", "p", 0, "Server port (overrides config)")
	rootCmd.PersistentFlags().StringVar(&serverHost, "host", "", "Server host (overrides config)")

	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(collectCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(versionCmd)
}
