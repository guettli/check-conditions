package cmd

import (
	"fmt"
	"os"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/guettli/check-conditions/pkg/checkconditions"
	"github.com/spf13/cobra"
)

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

var rootCmd = &cobra.Command{
	Use:     "check-conditions",
	Version: buildVersion(),
	Short:   "Check your cluster by looking at status.conditions of the resources",
	Long: `Check your cluster by looking at status.conditions of the resources

Output is usualy:

  namespace resource resource-name condition-type=condition-status condition-reason condition-message duration
`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if arguments.RetryCount == 0 {
			arguments.RetryForEver = true
		}
		for _, s := range ignoreConditionRegexStrings {
			r, err := regexp.Compile(s)
			if err != nil {
				return fmt.Errorf("invalid --ignore-condition-regex %q: %w", s, err)
			}
			arguments.ExtraConditionLinesToIgnoreRegexs = append(arguments.ExtraConditionLinesToIgnoreRegexs, r)
		}
		younger, err := checkconditions.ParseConditionTimeThreshold(ignoreConditionYoungerThanString)
		if err != nil {
			return fmt.Errorf("invalid --ignore-condition-younger-than: %w", err)
		}
		arguments.IgnoreConditionYoungerThan = younger
		older, err := checkconditions.ParseConditionTimeThreshold(ignoreConditionOlderThanString)
		if err != nil {
			return fmt.Errorf("invalid --ignore-condition-older-than: %w", err)
		}
		arguments.IgnoreConditionOlderThan = older
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

var ignoreConditionRegexStrings []string

var ignoreConditionYoungerThanString string
var ignoreConditionOlderThanString string

func init() {
	arguments.ProgrammStartTime = time.Now()
	rootCmd.Long = "check-conditions " + buildVersion() + "\n\n" + rootCmd.Long

	rootCmd.PersistentFlags().BoolVarP(&arguments.Verbose, "verbose", "v", false, "Create more output")

	rootCmd.PersistentFlags().DurationVarP(&arguments.Sleep, "sleep", "s", 15*time.Second, "Optional sleep duration (default: 5s)")

	rootCmd.PersistentFlags().DurationVarP(&arguments.Timeout, "timeout", "t", 0, "Optional timeout. When using 'all' or 'wait', this defines a timeout. Example: 5m for 5 minutes.")

	rootCmd.PersistentFlags().StringVarP(&arguments.Name, "name", "", "", "A string which will be printed in the output. Usefull if you have several terminals running the 'while' sub-command.")

	rootCmd.PersistentFlags().StringSliceVarP(&arguments.NamespacePatterns, "namespace", "n", nil, "Only check the given namespaces and skip cluster-scoped resources. Accepts a comma-separated list. Glob patterns (*, ?, [...]) are supported. Example: -n kube-system,kube-public or -n 'kube-*'")

	rootCmd.PersistentFlags().StringSliceVar(&arguments.ExcludeNamespacePatterns, "exclude-namespace", nil, "Skip the given namespaces. Accepts a comma-separated list. Glob patterns (*, ?, [...]) are supported. Combine with -n to subtract: -n 'foo-*' --exclude-namespace foo-bar checks every foo-* namespace except foo-bar.")

	rootCmd.PersistentFlags().Int16VarP(&arguments.RetryCount, "retry-count", "", 5, "Network errors: How many times to retry the command before giving up. This applies only to the first connection. As soon as a successful connection is made, the command will retry forever. Set to zero to also retry the first connection forever.")

	rootCmd.PersistentFlags().DurationVar(&arguments.WarnDeletionTimestampOlderThan, "warn-deletion-older-than", 10*time.Minute, "Warn about resources whose deletionTimestamp is older than this duration. Set to 0 to disable.")

	rootCmd.PersistentFlags().DurationVar(&arguments.PodStartGracePeriod, "pod-start-grace", 30*time.Second, "Treat a Pod whose ContainersReady/Initialized condition is False as healthy while it is still starting for the first time (no restarts) and younger than this duration. Set to 0 to disable.")

	rootCmd.PersistentFlags().Int64Var(&arguments.PodRestartWarnCount, "pod-restart-warn-count", 5, "Warn about a Pod container (regular, init or ephemeral) that has restarted at least this many times and is still unhealthy (waiting/CrashLoopBackOff or not ready). A container that starts again and again is not visible in status.conditions. Set to 0 to disable.")

	rootCmd.PersistentFlags().StringArrayVar(&ignoreConditionRegexStrings, "ignore-condition-regex", nil, "Additional regex to ignore a condition line, on top of the built-in ignore list. Can be given multiple times. Matched against: 'resource type=status reason \"message\"', e.g. 'mypods MyCondition=False MyReason .*'")

	rootCmd.PersistentFlags().StringVar(&ignoreConditionYoungerThanString, "ignore-condition-younger-than", "", "Ignore unhealthy conditions more recent than this. Accepts a duration relative to now (e.g. 5m to ignore conditions that changed within the last 5 minutes) or an RFC3339 timestamp (e.g. 2026-09-01T00:00:00Z to ignore conditions after that point in time). Empty disables it.")

	rootCmd.PersistentFlags().StringVar(&ignoreConditionOlderThanString, "ignore-condition-older-than", "", "Ignore unhealthy conditions older than this. Accepts a duration relative to now (e.g. 24h to ignore conditions that have not changed in the last day) or an RFC3339 timestamp (e.g. 2026-09-01T00:00:00Z to ignore conditions before that point in time). Empty disables it.")
}
