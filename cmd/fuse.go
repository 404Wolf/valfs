package cmd

import (
	"fmt"
	"log"
	"os"

	valfs "github.com/404wolf/valfs/fuse/valfs"
	"github.com/404wolf/valgo"
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

		configuration := valgo.NewConfiguration()
		configuration.AddDefaultHeader(
			"Authorization",
			"Bearer "+os.Getenv("VALTOWN_API_KEY"),
		)
		client := valgo.NewAPIClient(configuration)

		// Create a root node
		root := &valfs.ValFS{
			ValClient: client,
			MountDir:  directory,
		}

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
