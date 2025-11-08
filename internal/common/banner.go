package common

import (
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/banner"
)

// PrintBanner displays the application startup banner
func PrintBanner(config *Config, logger arbor.ILogger) {
	version := GetVersion()
	build := GetBuild()

	// Service URL
	serviceURL := fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.Port)

	// Create banner with custom styling - GREEN for quaero
	b := banner.New().
		SetStyle(banner.StyleDouble).
		SetBorderColor(banner.ColorGreen).
		SetTextColor(banner.ColorWhite).
		SetBold(true).
		SetWidth(80)

	// Visual banner still prints to stdout for startup aesthetics
	fmt.Printf("\n")
	b.PrintTopLine()
	b.PrintCenteredText("QUAERO")
	b.PrintCenteredText("Knowledge Search and Collection System")
	b.PrintSeparatorLine()
	b.PrintKeyValue("Version", version, 15)
	b.PrintKeyValue("Build", build, 15)
	b.PrintKeyValue("Environment", "development", 15)
	b.PrintKeyValue("Service URL", serviceURL, 15)
	b.PrintBottomLine()
	fmt.Printf("\n")

	// Log structured startup information through Arbor
	logger.Info().
		Str("version", version).
		Str("build", build).
		Str("environment", "development").
		Str("service_url", serviceURL).
		Str("config_file", "quaero.toml").
		Msg("Application started")

	// Print configuration details to console
	fmt.Printf("📋 Configuration:\n")
	fmt.Printf("   • Config File: quaero.toml\n")
	fmt.Printf("   • Web Interface: %s\n", serviceURL)

	// Show log file path if available
	logFilePath := ""
	// Try to get log file path if logger implements GetLogFilePath
	if loggerWithPath, ok := interface{}(logger).(interface{ GetLogFilePath() string }); ok {
		logFilePath = loggerWithPath.GetLogFilePath()
		if logFilePath != "" {
			fmt.Printf("   • Log File: %s\n", logFilePath)
		}
	}
	fmt.Printf("\n")

	// Log configuration through Arbor
	logger.Info().
		Str("log_file", logFilePath).
		Str("llm_mode", config.LLM.Mode).
		Str("storage_type", config.Storage.Type).
		Msg("Configuration loaded")

	// Print capabilities to console
	printCapabilities(config, logger)
	fmt.Printf("\n")
}

// printCapabilities displays the system capabilities
func printCapabilities(config *Config, logger arbor.ILogger) {
	fmt.Printf("🎯 Enabled Features:\n")

	// Show crawler integration
	fmt.Printf("   • Generic web crawler (ChromeDP-based)\n")

	// Show storage configuration
	fmt.Printf("   • Local SQLite database with full-text search\n")

	// Show LLM mode
	llmDescription := ""
	if config.LLM.Mode == "offline" {
		llmDescription = "secure, data stays local"
		fmt.Printf("   • Offline LLM mode (%s)\n", llmDescription)
	} else if config.LLM.Mode == "cloud" {
		llmDescription = "uses external APIs"
		fmt.Printf("   • Cloud LLM mode (%s)\n", llmDescription)
	}

	// Show authentication
	fmt.Printf("   • Extension-based authentication (OAuth/SSO)\n")

	// Log capabilities through Arbor
	logger.Info().
		Str("storage", "sqlite_fts5").
		Str("llm_mode", config.LLM.Mode).
		Str("llm_description", llmDescription).
		Str("authentication", "extension_oauth_sso").
		Str("crawler", "chromedp").
		Msg("System capabilities")
}

// PrintShutdownBanner displays the application shutdown banner
func PrintShutdownBanner(logger arbor.ILogger) {
	b := banner.New().
		SetStyle(banner.StyleDouble).
		SetBorderColor(banner.ColorGreen).
		SetTextColor(banner.ColorWhite).
		SetBold(true).
		SetWidth(42)

	// Visual banner to stdout
	b.PrintTopLine()
	b.PrintCenteredText("SHUTTING DOWN")
	b.PrintCenteredText("QUAERO")
	b.PrintBottomLine()
	fmt.Println()

	// Log shutdown through Arbor
	logger.Info().Msg("Application shutting down")
}

// PrintColorizedMessage prints a message with specified color and logs through Arbor
func PrintColorizedMessage(color, message string, logger arbor.ILogger) {
	fmt.Printf("%s%s%s\n", color, message, banner.ColorReset)
}

// PrintSuccess prints a success message in green and logs it
func PrintSuccess(message string) {
	logger := GetLogger()
	PrintColorizedMessage(banner.ColorGreen, fmt.Sprintf("✓ %s", message), logger)
	logger.Info().Str("type", "success").Msg(message)
}

// PrintError prints an error message in red and logs it
func PrintError(message string) {
	logger := GetLogger()
	PrintColorizedMessage(banner.ColorRed, fmt.Sprintf("✗ %s", message), logger)
	logger.Error().Str("type", "error").Msg(message)
}

// PrintWarning prints a warning message in yellow and logs it
func PrintWarning(message string) {
	logger := GetLogger()
	PrintColorizedMessage(banner.ColorYellow, fmt.Sprintf("⚠ %s", message), logger)
	logger.Warn().Str("type", "warning").Msg(message)
}

// PrintInfo prints an info message in cyan and logs it
func PrintInfo(message string) {
	logger := GetLogger()
	PrintColorizedMessage(banner.ColorCyan, fmt.Sprintf("ℹ %s", message), logger)
	logger.Info().Str("type", "info").Msg(message)
}
