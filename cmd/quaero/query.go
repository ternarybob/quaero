package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query [question]",
	Short: "Ask a natural language question",
	Long:  `Query the knowledge base using natural language. Returns an answer based on collected documentation.`,
	Args:  cobra.ExactArgs(1),
	Run:   runQuery,
}

var (
	queryIncludeSources bool
	queryIncludeImages  bool
)

func init() {
	queryCmd.Flags().BoolVar(&queryIncludeSources, "sources", false, "Include source references in answer")
	queryCmd.Flags().BoolVar(&queryIncludeImages, "images", false, "Process relevant images")
}

func runQuery(cmd *cobra.Command, args []string) {
	question := args[0]

	logger.Info().Str("question", question).Msg("Searching for question")

	// TODO: Implement query
	logger.Warn().Msg("Query implementation pending")
	fmt.Println("\nQuery implementation pending\n")
}
