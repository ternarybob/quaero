// -----------------------------------------------------------------------
// Last Modified: Friday, 8th November 2025 4:00:00 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/arbor/models"
	"github.com/ternarybob/quaero/internal/app"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/server"
)

// configPaths is a custom flag type that allows multiple -config flags
type configPaths []string

func (c *configPaths) String() string {
	return fmt.Sprintf("%v", *c)
}

func (c *configPaths) Set(value string) error {
	*c = append(*c, value)
	return nil
}

var (
	// Command-line flags
	configFiles  configPaths // Multiple -config flags supported
	serverPort   = flag.Int("port", 0, "Server port (overrides config)")
	serverPortP  = flag.Int("p", 0, "Server port (shorthand, overrides config)")
	serverHost   = flag.String("host", "", "Server host (overrides config)")
	showVersion  = flag.Bool("version", false, "Print version information")
	showVersionV = flag.Bool("v", false, "Print version information (shorthand)")

	// Global state
	config *common.Config
	logger arbor.ILogger
)

func init() {
	// Register custom flag for multiple config files
	flag.Var(&configFiles, "config", "Configuration file path (can be specified multiple times, later files override earlier ones)")
	flag.Var(&configFiles, "c", "Configuration file path (shorthand)")
}

func main() {
	// Parse command-line flags
	flag.Parse()

	// Handle version flag
	if *showVersion || *showVersionV {
		fmt.Printf("Quaero version %s\n", common.GetVersion())
		os.Exit(0)
	}

	// Merge port flags (shorthand takes precedence)
	finalPort := *serverPort
	if *serverPortP != 0 {
		finalPort = *serverPortP
	}

	// Startup sequence (REQUIRED ORDER):
	// 1. Load config (defaults -> file1 -> file2 -> ... -> env)
	// 2. Apply CLI overrides (highest priority)
	// 3. Initialize logger
	// 4. Print banner
	var err error

	// Auto-discover config file if not specified
	if len(configFiles) == 0 {
		// Check current directory first
		if _, err := os.Stat("quaero.toml"); err == nil {
			configFiles = append(configFiles, "quaero.toml")
		} else if _, err := os.Stat("deployments/local/quaero.toml"); err == nil {
			// Fallback: check deployments/local for users running from project root
			configFiles = append(configFiles, "deployments/local/quaero.toml")
		}
	}

	// 1. Load configuration (default -> file1 -> file2 -> ... -> env -> CLI)
	// Later config files override earlier ones
	config, err = common.LoadFromFiles(configFiles...)
	if err != nil {
		// Use temporary logger for startup errors
		tempLogger := arbor.NewLogger()
		if len(configFiles) == 0 {
			tempLogger.Fatal().Err(err).Msg("Failed to load configuration: no config file found")
		} else {
			tempLogger.Fatal().Strs("paths", configFiles).Err(err).Msg("Failed to load configuration files")
		}
		os.Exit(1)
	}

	// 2. Apply command-line flag overrides (highest priority)
	common.ApplyFlagOverrides(config, finalPort, *serverHost)

	// 3. Initialize logger with final configuration (inline from common.InitLogger)
	logger = arbor.NewLogger()

	// Get executable path for log directory
	execPath, err := os.Executable()
	if err != nil {
		// Add console writer first, then log the warning
		logger = logger.WithConsoleWriter(models.WriterConfiguration{
			Type:             models.LogWriterTypeConsole,
			TimeFormat:       "15:04:05",
			TextOutput:       true,
			DisableTimestamp: false,
		})
		logger.Warn().Err(err).Msg("Failed to get executable path - using fallback console logging")
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
				// Use console writer temporarily for this warning
				tempLogger := logger.WithConsoleWriter(models.WriterConfiguration{
					Type:             models.LogWriterTypeConsole,
					TimeFormat:       "15:04:05",
					TextOutput:       true,
					DisableTimestamp: false,
				})
				tempLogger.Warn().Err(err).Str("logs_dir", logsDir).Msg("Failed to create logs directory")
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
			logger.Warn().
				Strs("configured_outputs", config.Logging.Output).
				Msg("No visible log outputs configured - falling back to console")
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
		Str("log_level", config.Logging.Level).
		Strs("log_output", config.Logging.Output).
		Bool("crawler_enabled", true).
		Msg("Resolved configuration (sanitized)")

	// Log initialization complete
	logger.Info().
		Strs("config_files", configFiles).
		Int("port", config.Server.Port).
		Str("host", config.Server.Host).
		Msg("Application configuration loaded")

	// Start server
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
		defer func() {
			if r := recover(); r != nil {
				logger.Fatal().Str("panic", fmt.Sprintf("%v", r)).Msg("Server goroutine panicked")
			}
		}()

		if err := srv.Start(); err != nil {
			logger.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Give goroutine a moment to start
	time.Sleep(100 * time.Millisecond)

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
