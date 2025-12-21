package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
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

		// Set mode: priority is flag > env var > legacy env var > default
		if modeFlag != "" {
			arguments.Mode = checkconditions.ConditionMode(strings.ToLower(modeFlag))
		} else {
			envMode := os.Getenv("CHECK_CONDITIONS_MODE")
			if envMode != "" {
				arguments.Mode = checkconditions.ConditionMode(strings.ToLower(envMode))
			} else {
				// Check legacy environment variable for backward compatibility
				if os.Getenv("CHECK_CONDITIONS_COMPARE_WITH_NEW_CONFIG") != "" {
					arguments.Mode = checkconditions.ModeOldCompareNew
				} else {
					arguments.Mode = checkconditions.ModeOnlyOld
				}
			}
		}

		// Validate mode
		validModes := map[checkconditions.ConditionMode]bool{
			checkconditions.ModeOnlyOld:       true,
			checkconditions.ModeOnlyNew:       true,
			checkconditions.ModeOldCompareNew: true,
			checkconditions.ModeNewCompareOld: true,
		}
		if !validModes[arguments.Mode] {
			return fmt.Errorf("invalid mode %q: must be one of: only-old, only-new, old-compare-new, new-compare-old", arguments.Mode)
		}

		cfg, path, err := checkconditions.LoadConfig(arguments.SkipBuiltinConfig)
		if err != nil {
			if errors.Is(err, checkconditions.ErrInvalidConfigYAML) {
				cmd.SilenceUsage = true
			}
			return err
		}

		if cfg == nil {
			cfg = &checkconditions.Config{}
		}

		if arguments.AutoAddFromLegacyConfig && path == "" {
			newPath, created, err := checkconditions.EnsureConfigPath()
			if err != nil {
				return err
			}
			if created {
				fmt.Printf("Created config at %s\n", newPath)
			}
			cfg.SetPath(newPath)
			path = newPath
		}

		if path != "" {
			cfg.SetPath(path)
		}

		if arguments.Verbose && !arguments.SkipBuiltinConfig {
			fmt.Println("Loaded built-in config")
		}
		if arguments.Verbose && cfg != nil && path != "" {
			fmt.Printf("Loaded config from %s\n", path)
		}

		arguments.Config = cfg
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

var arguments = checkconditions.Arguments{}
var modeFlag string

func init() {
	arguments.ProgrammStartTime = time.Now()

	rootCmd.PersistentFlags().BoolVarP(&arguments.Verbose, "verbose", "v", false, "Create more output")

	rootCmd.PersistentFlags().DurationVarP(&arguments.Sleep, "sleep", "s", 15*time.Second, "Optional sleep duration (default: 5s)")

	rootCmd.PersistentFlags().DurationVarP(&arguments.Timeout, "timeout", "t", 0, "Optional timeout. When using 'all' or 'wait', this defines a timeout. Example: 5m for 5 minutes.")

	rootCmd.PersistentFlags().StringVarP(&arguments.Name, "name", "", "", "A string which will be printed in the output. Usefull if you have several terminals running the 'while' sub-command.")

	rootCmd.PersistentFlags().StringVarP(&arguments.Namespace, "namespace", "n", "", "Only check the given namespace and skip cluster-scoped resources")

	rootCmd.PersistentFlags().Int16VarP(&arguments.RetryCount, "retry-count", "", 5, "Network errors: How many times to retry the command before giving up. This applies only to the first connection. As soon as a successful connection is made, the command will retry forever. Set to zero to also retry the first connection forever.")

	rootCmd.PersistentFlags().StringVar(&modeFlag, "mode", "", "Condition classification mode: only-old (default), only-new, old-compare-new, new-compare-old. Can also be set via CHECK_CONDITIONS_MODE environment variable.")

	rootCmd.PersistentFlags().BoolVar(&arguments.AutoAddFromLegacyConfig, "auto-add-from-legacy-config", false, "Automatically append entries that the legacy logic would ignore.")

	rootCmd.PersistentFlags().BoolVar(&arguments.SkipBuiltinConfig, "skip-loading-built-in-config", false, "Skip loading the embedded built-in condition config.")
}
