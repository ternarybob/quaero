package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ternarybob/quaero/internal/common"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Quaero version %s\n", common.GetVersion())
	},
}
