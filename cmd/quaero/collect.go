package main

import (
	"log"

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
		log.Println("Collecting from all sources...")
		// TODO: Implement collection
		log.Println("Collection implementation pending")
	} else if collectSource != "" {
		log.Printf("Collecting from %s...\n", collectSource)
		// TODO: Implement collection
		log.Println("Collection implementation pending")
	} else {
		log.Println("Please specify --source or --all")
		cmd.Help()
	}
}
