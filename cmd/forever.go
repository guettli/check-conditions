package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/guettli/check-conditions/pkg/checkconditions"
	"github.com/spf13/cobra"
)

var foreverCmd = &cobra.Command{
	Use:   "forever",
	Short: "Check all conditions of all api-resources, repeat forever.",
	Args:  cobra.MatchAll(cobra.MaximumNArgs(0)),
	Run: func(cmd *cobra.Command, args []string) {
		// RunForever loops until it hits an error, so it never returns nil.
		err := checkconditions.RunForever(context.Background(), &arguments)
		fmt.Println(err)
		os.Exit(3)
	},
}

func init() {
	rootCmd.AddCommand(foreverCmd)
}
