package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "valfs",
	Short: "Mount your Val.Town vals as a file system",
	Long:  "Mount your Val.Town vals as a file system of Vals that you can read, modfiy, and run, remotely, on the Val.Town runtime.",
}

func InitRoot() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	FuseInit()
	ValsInit()
}

func Execute() error {
	InitRoot()
	return rootCmd.Execute()
}
