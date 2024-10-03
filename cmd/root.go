package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "valfs",
	Short: "Mount your Val.Town vals as a file system",
	Long:  `Mount your Val.Town vals as a file system of Vals that you can read, modfiy, and run, remotely, on the Val.Town runtime.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
