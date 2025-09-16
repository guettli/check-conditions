package cmd

import (
	"os"
	"time"

	"github.com/guettli/check-conditions/pkg/checkconditions"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "check-conditions",
	Short: "Check your cluster by looking at status.conditions of the resources",
	Long: `Check your cluster by looking at status.conditions of the resources

Output is usualy:

  namespace resource resource-name condition-type=condition-status condition-reason condition-message duration
`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if arguments.RetryCount == 0 {
			arguments.RetryForEver = true
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var arguments = checkconditions.Arguments{
	Stdout: os.Stdout,
}

func init() {
	arguments.ProgrammStartTime = time.Now()

	rootCmd.PersistentFlags().BoolVarP(&arguments.Verbose, "verbose", "v", false, "Create more output")

	rootCmd.PersistentFlags().DurationVarP(&arguments.Sleep, "sleep", "s", 15*time.Second, "Optional sleep duration (default: 5s)")

	rootCmd.PersistentFlags().DurationVarP(&arguments.Timeout, "timeout", "t", 0, "Optional timeout. When using 'all' or 'wait', this defines a timeout. Example: 5m for 5 minutes.")

	rootCmd.PersistentFlags().StringVarP(&arguments.Name, "name", "", "", "A string which will be printed in the output. Usefull if you have several terminals running the 'while' sub-command.")

	rootCmd.PersistentFlags().Int16VarP(&arguments.RetryCount, "retry-count", "", 5, "Network errors: How many times to retry the command before giving up. This applies only to the first connection. As soon as a successful connection is made, the command will retry forever. Set to zero to also retry the first connection forever.")
}
