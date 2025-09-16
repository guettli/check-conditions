package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/guettli/check-conditions/pkg/checkconditions"
	"github.com/spf13/cobra"
)

var whileCmd = &cobra.Command{
	Use:   "while your-regex",
	Short: "Check all conditions of all api-resources, repeat until regex does not match anymore. Use '.' to wait until all conditions are healthy.",
	Args:  cobra.MatchAll(cobra.MaximumNArgs(1)),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("Please provide exactly one argument: your-regex")
			os.Exit(3)
		}

		r, err := regexp.Compile(args[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		arguments.WhileRegex = r

		err = checkconditions.RunWhileRegex(context.Background(), &arguments)
		if err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(whileCmd)
}
