package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/guettli/check-conditions/pkg/checkconditions"
	"github.com/spf13/cobra"
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Check all conditions of all api-resources",
	Long:  `...`,
	Run: func(cmd *cobra.Command, args []string) {
		unhealthy, err := checkconditions.RunAllOnce(context.Background(), arguments)
		if err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		if unhealthy {
			os.Exit(1)
		}
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(allCmd)
}
