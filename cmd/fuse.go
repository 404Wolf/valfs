package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	client "github.com/404wolf/valfs/client"

	valfs "github.com/404wolf/valfs/fuse/valfs"
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

		// Create a root node
		root := valfs.NewValFS(
			directory,
			client.NewClient(os.Getenv("VALTOWN_API_KEY"), context.Background()),
		)

		fmt.Println("Mounting ValFS file system at", directory)
		if err := root.Mount(); err != nil {
			log.Fatalf("Mount failed: %v", err)
		}
	},
}

func FuseInit() {
	fuseCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(fuseCmd)
}
