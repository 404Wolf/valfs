package cmd

import (
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
	Long:  "Mount your Val.Town vals as a file system of Vals that you can read, modfiy, and run, remotely, on the Val.Town runtime.",
}

func InitRoot() {
	cobra.OnInitialize(initLogger)
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	FuseInit()
	ValsInit()
}

func Execute() error {
	InitRoot()
	return rootCmd.Execute()
}
