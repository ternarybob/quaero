package common

import (
	"fmt"

	"github.com/ternarybob/banner"
)

// PrintBanner displays the application startup banner
func PrintBanner(config *Config, logger interface{ GetLogFilePath() string }) {
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

	fmt.Printf("\n")

	// Print banner header
	b.PrintTopLine()
	b.PrintCenteredText("QUAERO")
	b.PrintCenteredText("Knowledge Search and Collection System")
	b.PrintSeparatorLine()

	// Print version and runtime information
	b.PrintKeyValue("Version", version, 15)
	b.PrintKeyValue("Build", build, 15)
	b.PrintKeyValue("Environment", "development", 15)
	b.PrintKeyValue("Service URL", serviceURL, 15)
	b.PrintBottomLine()

	fmt.Printf("\n")

	// Print configuration details
	fmt.Printf("📋 Configuration:\n")
	fmt.Printf("   • Config File: quaero.toml\n")
	fmt.Printf("   • Web Interface: %s\n", serviceURL)

	// Show log file path if file output is enabled
	if logger != nil {
		logFilePath := logger.GetLogFilePath()
		if logFilePath != "" {
			fmt.Printf("   • Log File: %s\n", logFilePath)
		}
	}
	fmt.Printf("\n")

	// Print capabilities
	printCapabilities(config)
	fmt.Printf("\n")
}

// printCapabilities displays the system capabilities
func printCapabilities(config *Config) {
	fmt.Printf("🎯 Enabled Features:\n")

	// Show enabled sources
	sourcesEnabled := false
	if config.Sources.Jira.Enabled {
		fmt.Printf("   • Jira integration (projects and issues)\n")
		sourcesEnabled = true
	}
	if config.Sources.Confluence.Enabled {
		fmt.Printf("   • Confluence integration (spaces and pages)\n")
		sourcesEnabled = true
	}
	if config.Sources.GitHub.Enabled {
		fmt.Printf("   • GitHub integration (repositories)\n")
		sourcesEnabled = true
	}
	if !sourcesEnabled {
		fmt.Printf("   • No data sources enabled (configure in quaero.toml)\n")
	}

	// Show storage configuration
	fmt.Printf("   • Local SQLite database with full-text search\n")

	// Show LLM mode
	if config.LLM.Mode == "offline" {
		fmt.Printf("   • Offline LLM mode (secure, data stays local)\n")
	} else if config.LLM.Mode == "cloud" {
		fmt.Printf("   • Cloud LLM mode (uses external APIs)\n")
	}

	// Show authentication
	fmt.Printf("   • Extension-based authentication (OAuth/SSO)\n")
}

// PrintShutdownBanner displays the application shutdown banner
func PrintShutdownBanner() {
	b := banner.New().
		SetStyle(banner.StyleDouble).
		SetBorderColor(banner.ColorGreen).
		SetTextColor(banner.ColorWhite).
		SetBold(true).
		SetWidth(42)

	b.PrintTopLine()
	b.PrintCenteredText("SHUTTING DOWN")
	b.PrintCenteredText("QUAERO")
	b.PrintBottomLine()
	fmt.Println()
}

// PrintColorizedMessage prints a message with specified color
func PrintColorizedMessage(color, message string) {
	fmt.Printf("%s%s%s\n", color, message, banner.ColorReset)
}

// PrintSuccess prints a success message in green
func PrintSuccess(message string) {
	PrintColorizedMessage(banner.ColorGreen, fmt.Sprintf("✓ %s", message))
}

// PrintError prints an error message in red
func PrintError(message string) {
	PrintColorizedMessage(banner.ColorRed, fmt.Sprintf("✗ %s", message))
}

// PrintWarning prints a warning message in yellow
func PrintWarning(message string) {
	PrintColorizedMessage(banner.ColorYellow, fmt.Sprintf("⚠ %s", message))
}

// PrintInfo prints an info message in cyan
func PrintInfo(message string) {
	PrintColorizedMessage(banner.ColorCyan, fmt.Sprintf("ℹ %s", message))
}
