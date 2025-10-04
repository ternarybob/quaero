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
	fmt.Printf("🎯 Quaero Capabilities:\n")
	fmt.Printf("   • Extension-based authentication (OAuth/SSO compatible)\n")

	// Show enabled sources
	if config.Sources.Jira.Enabled {
		fmt.Printf("   • Jira project and issue scraping\n")
	}
	if config.Sources.Confluence.Enabled {
		fmt.Printf("   • Confluence space and page scraping\n")
	}
	if config.Sources.GitHub.Enabled {
		fmt.Printf("   • GitHub repository scraping\n")
	}

	fmt.Printf("   • Local BoltDB storage\n")
	fmt.Printf("   • Rate-limited API requests\n")
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
