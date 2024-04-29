package cmd

import (
	"github.com/guettli/check-conditions/pkg/checkconditions"
	"github.com/spf13/cobra"
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Check all conditions of all api-resources",
	Long:  `...`,
	Run: func(cmd *cobra.Command, args []string) {
		checkconditions.RunAll(arguments)
	},
}

func init() {
	rootCmd.AddCommand(allCmd)
}
