package cmd

import (
	"os"

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
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var arguments = checkconditions.Arguments{}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().BoolVarP(&arguments.Verbose, "verbose", "v", false, "Create more output")
}
