package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	common "github.com/404wolf/valfs/common"

	valfs "github.com/404wolf/valfs/fuse/valfs"
	"github.com/spf13/cobra"
)

var fuseCmd = &cobra.Command{
	Short: "Fuse related actions",
}

var noRefresh bool

var mountCmd = &cobra.Command{
	Use:   "mount <directory>",
	Short: "Mount your Vals to a directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		directory := args[0]

		// Create a root node
		root := valfs.NewValFS(
			directory,
			common.NewClient(
				os.Getenv("VAL_TOWN_API_KEY"),
				context.Background(),
				!noRefresh, // Use the opposite of noRefresh
			),
		)

		fmt.Println("Mounting ValFS file system at", directory)
		if err := root.Mount(func() {}); err != nil {
			log.Fatalf("Mount failed: %v", err)
		}
	},
}

func FuseInit() {
	mountCmd.Flags().BoolVar(&noRefresh, "no-refresh", false, "Disable automatic refreshing")
	fuseCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(fuseCmd)
}
