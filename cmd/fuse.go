package cmd

import (
	"fmt"
	"log"

	"github.com/404wolf/valfs/fuse"
	"github.com/404wolf/valfs/sdk"
	"github.com/spf13/cobra"
)

var fuseCmd = &cobra.Command{
	Short: "Fuse related actions",
}

var mountCmd = &cobra.Command{
	Use:   "mount <directory>",
	Short: "Mount your Vals to a directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		directory := args[0]

		client, err := sdk.NewValTownClient()
		if err != nil {
			log.Fatal(err)
		}
		valFs := fuse.NewValFS(client, directory)

		fmt.Println("Mounting ValFS file system at", directory)
		valFs.Mount()
	},
}

func FuseInit() {
	fuseCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(fuseCmd)
}
