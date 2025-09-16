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
		ctx := context.Background()
		err := arguments.InitClients(ctx)
		if err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		unhealthy, err := checkconditions.RunAllOnce(ctx, &arguments)
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
