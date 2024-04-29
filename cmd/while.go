package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/guettli/check-conditions/pkg/checkconditions"
	"github.com/spf13/cobra"
)

var whileCmd = &cobra.Command{
	Use:   "while your-regex",
	Short: "Check all conditions of all api-resources, repeat until regex does not match anymore.",
	Long:  `Check conditions until the give regex does not match anymore.`,
	Args:  cobra.MatchAll(cobra.MaximumNArgs(1)),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			r, err := regexp.Compile(args[0])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			arguments.WhileRegex = r
		} else {
			arguments.WhileForever = true
		}
		checkconditions.RunAll(arguments)
	},
}

func init() {
	rootCmd.AddCommand(whileCmd)
}
