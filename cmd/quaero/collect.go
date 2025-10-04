package main

import (
	"github.com/spf13/cobra"
)

var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Manually trigger collection from sources",
	Long:  `Triggers data collection from configured sources (Confluence, Jira, etc.)`,
	Run:   runCollect,
}

var (
	collectSource string
	collectAll    bool
)

func init() {
	collectCmd.Flags().StringVar(&collectSource, "source", "", "Specific source to collect from (confluence, jira, github)")
	collectCmd.Flags().BoolVar(&collectAll, "all", false, "Collect from all sources")
}

func runCollect(cmd *cobra.Command, args []string) {
	if collectAll {
		logger.Info().Msg("Collecting from all sources")
		// TODO: Implement collection
		logger.Warn().Msg("Collection implementation pending")
	} else if collectSource != "" {
		logger.Info().Str("source", collectSource).Msg("Collecting from source")
		// TODO: Implement collection
		logger.Warn().Msg("Collection implementation pending")
	} else {
		logger.Error().Msg("Please specify --source or --all")
		cmd.Help()
	}
}
