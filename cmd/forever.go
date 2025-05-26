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
		err := checkconditions.RunForever(context.Background(), &arguments)
		if err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(foreverCmd)
}
