package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	verbose bool
	logFile string
)

var rootCmd = &cobra.Command{
	Use:   "valfs",
	Short: "Mount your Val.Town vals as a file system",
	Long:  "Mount your Val.Town vals as a file system of Vals that you can read, modify, and run, remotely, on the Val.Town runtime.",
}

func handleShebangCall(args []string) {
	scriptPath := args[1]
	scriptArgs := args[2:]

	fmt.Printf("Running script: %s\n", scriptPath)
	fmt.Printf("Script arguments: %v\n", scriptArgs)
}

func InitRoot() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "log file path")

	ValfsInit()
}

func Execute(logger *zap.SugaredLogger) error {
	InitRoot()
	return rootCmd.Execute()
}
