package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	logger  *log.Logger
	verbose bool
)

func initLogger() {
	if verbose {
		log.SetOutput(os.Stdout)
	} else {
		log.SetOutput(io.Discard)
	}
}

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
	cobra.OnInitialize(initLogger)
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	ValfsInit()
}

func Execute(logger *zap.SugaredLogger) error {
	InitRoot()
	return rootCmd.Execute()
}
