package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

var (
	configPath string
	config     *common.Config
	logger     *arbor.Logger
)

var rootCmd = &cobra.Command{
	Use:   "quaero",
	Short: "Quaero - Knowledge search system",
	Long:  `Quaero (Latin: "I seek") - A local knowledge base system with natural language queries.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize configuration
		var err error
		config, err = common.LoadFromFile(configPath)
		if err != nil {
			logger = arbor.New()
			logger.Fatal("Failed to load configuration", "error", err, "path", configPath)
		}

		// Initialize logger with config
		logger = common.InitLogger(config)
	},
}

func main() {
	// Print banner
	common.PrintBanner(common.GetVersion())

	if err := rootCmd.Execute(); err != nil {
		if logger != nil {
			logger.Fatal("Command execution failed", "error", err)
		}
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.toml", "Configuration file path")

	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(collectCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(versionCmd)
}
