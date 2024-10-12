package cmd

import (
	"github.com/404wolf/valfs/fuse"
	"github.com/404wolf/valfs/sdk"
	"github.com/spf13/cobra"
	"log"
)

var fuseCmd = &cobra.Command{
	Short: "Fuse related actions",
}

var mountCmd = &cobra.Command{
	Use:   "mount <directory> <userId>",
	Short: "Mount your Vals to a directory",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		directory := args[0]
		userId := args[1]

		client, err := sdk.NewClient()
		if err != nil {
			log.Fatal(err)
		}
		valFs := fuse.NewValFS(client, userId, directory)

		log.Println("Mounting ValFS file system")
		valFs.Mount()
	},
}

func FuseInit() {
	fuseCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(fuseCmd)
}
