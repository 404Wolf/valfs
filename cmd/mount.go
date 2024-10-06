package cmd

import (
	"github.com/404wolf/valfs/fuse"
	"github.com/404wolf/valfs/sdk"
	"github.com/spf13/cobra"
	"log"
)

var mountCmd = &cobra.Command{
	Use:   "mount [directory]",
	Short: "Mount your Val.Town Vals to a directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := sdk.NewClient()
		if err != nil {
			log.Fatal(err)
		}
		valFs := fuse.NewValFS(client, &sdk.ValAuthor{Username: "404wolf"}, args[0])

		log.Println("Mounting ValFS file system")
		valFs.Mount()
	},
}

func init() {
	rootCmd.AddCommand(mountCmd)
}
