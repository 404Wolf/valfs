package cmd

import (
	"fmt"

	"github.com/404wolf/valfs/common"
	"github.com/spf13/cobra"
)

var (
	logFile  string
	logLevel string
	silent   bool
)

// validateAndSetupLogging validates the log level and sets up the logger.
// Returns an error if the log level is invalid.
func validateAndSetupLogging() error {
	validLevels := []string{"debug", "info", "warn", "error"}
	for _, valid := range validLevels {
		if logLevel == valid {
			common.Logger = common.SetupLogger(logFile, logLevel, silent)
			return nil
		}
	}
	return fmt.Errorf("invalid log level: %s. Valid levels are: debug, info, warn, error", logLevel)
}

var rootCmd = &cobra.Command{
	Use:   "valfs",
	Short: "Mount your Val.Town vals as a file system",
	Long:  "Mount your Val.Town vals as a file system of Vals that you can read, modify, and run, remotely, on Val.Town",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return validateAndSetupLogging()
	},
}

func handleShebangCall(args []string) {
	scriptPath := args[1]
	scriptArgs := args[2:]

	fmt.Printf("Running script: %s\n", scriptPath)
	fmt.Printf("Script arguments: %v\n", scriptArgs)
}

func InitRoot() {
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "log file path")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "logging level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&silent, "silent", false, "disable stdout logging")

	ValfsInit()
}

func Execute() error {
	InitRoot()
	return rootCmd.Execute()
}
