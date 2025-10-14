// -----------------------------------------------------------------------
// Last Modified: Tuesday, 14th October 2025 12:40:41 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/arbor/models"
	"github.com/ternarybob/quaero/internal/app"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/server"
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
	Run:   runServer,
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

		// 3. Initialize logger with final configuration (inline from common.InitLogger)
		logger = arbor.NewLogger()

		// Get executable path for log directory
		execPath, err := os.Executable()
		if err != nil {
			fmt.Printf("Warning: Failed to get executable path: %v\n", err)
			logger = logger.WithConsoleWriter(models.WriterConfiguration{
				Type:             models.LogWriterTypeConsole,
				TimeFormat:       "15:04:05",
				TextOutput:       true,
				DisableTimestamp: false,
			})
		} else {
			execDir := filepath.Dir(execPath)
			logsDir := filepath.Join(execDir, "logs")

			// Check if file output is enabled
			hasFileOutput := false
			hasStdoutOutput := false
			for _, output := range config.Logging.Output {
				if output == "file" {
					hasFileOutput = true
				}
				if output == "stdout" || output == "console" {
					hasStdoutOutput = true
				}
			}

			// Configure file logging if enabled
			if hasFileOutput {
				if err := os.MkdirAll(logsDir, 0755); err != nil {
					fmt.Printf("Warning: Failed to create logs directory: %v\n", err)
				} else {
					logFile := filepath.Join(logsDir, "quaero.log")
					logger = logger.WithFileWriter(models.WriterConfiguration{
						Type:             models.LogWriterTypeFile,
						FileName:         logFile,
						TimeFormat:       "15:04:05",
						MaxSize:          100 * 1024 * 1024, // 100 MB
						MaxBackups:       3,
						TextOutput:       true,
						DisableTimestamp: false,
					})
				}
			}

			// Configure console logging if enabled
			if hasStdoutOutput {
				logger = logger.WithConsoleWriter(models.WriterConfiguration{
					Type:             models.LogWriterTypeConsole,
					TimeFormat:       "15:04:05",
					TextOutput:       true,
					DisableTimestamp: false,
				})
			}

			// Ensure at least one visible log writer is configured
			if !hasFileOutput && !hasStdoutOutput {
				logger = logger.WithConsoleWriter(models.WriterConfiguration{
					Type:             models.LogWriterTypeConsole,
					TimeFormat:       "15:04:05",
					TextOutput:       true,
					DisableTimestamp: false,
				})
				fmt.Printf("Warning: No visible log outputs configured, falling back to console\n")
			}
		}

		// Always add memory writer for WebSocket log streaming
		logger = logger.WithMemoryWriter(models.WriterConfiguration{
			Type:             models.LogWriterTypeMemory,
			TimeFormat:       "15:04:05",
			TextOutput:       true,
			DisableTimestamp: false,
		})

		// Set log level
		logger = logger.WithLevelFromString(config.Logging.Level)

		// Store logger in singleton for global access
		common.InitLogger(logger)

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

func runServer(cmd *cobra.Command, args []string) {
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

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().IntVarP(&serverPort, "port", "p", 0, "Server port (overrides config)")
	rootCmd.PersistentFlags().StringVar(&serverHost, "host", "", "Server host (overrides config)")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
}
