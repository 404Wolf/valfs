package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"log"
	"os"
)

var logger *log.Logger
var verbose bool

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
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	FuseInit()
	ValsInit()
}

func Execute() error {
	InitRoot()
	return rootCmd.Execute()
}
