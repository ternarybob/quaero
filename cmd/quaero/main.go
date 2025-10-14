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
		// Startup sequence (REQUIRED ORDER):
		// 1. Load config (defaults -> file -> env)
		// 2. Apply CLI overrides (highest priority)
		// 3. Initialize logger
		// 4. Print banner
		var err error

		// Auto-discover config file if not specified
		if configPath == "" {
			// Check current directory first
			if _, err := os.Stat("quaero.toml"); err == nil {
				configPath = "quaero.toml"
			} else if _, err := os.Stat("deployments/local/quaero.toml"); err == nil {
				// Fallback: check deployments/local for users running from project root
				configPath = "deployments/local/quaero.toml"
			}
		}

		// 1. Load configuration (default -> file -> env -> CLI)
		config, err = common.LoadFromFile(configPath)
		if err != nil {
			// Use temporary logger for startup errors
			tempLogger := arbor.NewLogger()
			if configPath == "" {
				tempLogger.Fatal().Err(err).Msg("Failed to load configuration: no config file found")
			} else {
				tempLogger.Fatal().Str("path", configPath).Err(err).Msg("Failed to load configuration file")
			}
			os.Exit(1)
		}

		// 2. Apply CLI flag overrides (highest priority)
		common.ApplyCLIOverrides(config, serverPort, serverHost)

		// 3. Initialize logger with final configuration
		logger = common.InitLogger(config)

		// 4. Print banner with configuration and logger
		common.PrintBanner(config, logger)

		// Debug: Log final resolved configuration for troubleshooting
		logger.Debug().
			Str("storage_type", config.Storage.Type).
			Str("sqlite_path", config.Storage.SQLite.Path).
			Str("jira_enabled", fmt.Sprintf("%v", config.Sources.Jira.Enabled)).
			Str("confluence_enabled", fmt.Sprintf("%v", config.Sources.Confluence.Enabled)).
			Str("github_enabled", fmt.Sprintf("%v", config.Sources.GitHub.Enabled)).
			Str("log_level", config.Logging.Level).
			Strs("log_output", config.Logging.Output).
			Msg("Resolved configuration (sanitized)")

		// Log initialization complete
		logger.Info().
			Str("config_path", configPath).
			Int("port", config.Server.Port).
			Str("host", config.Server.Host).
			Str("llm_mode", config.LLM.Mode).
			Msg("Application configuration loaded")
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
	rootCmd.AddCommand(versionCmd)
}
