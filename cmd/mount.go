package cmd

import (
	"context"
	"log"
	"os"

	common "github.com/404wolf/valfs/common"
	valfs "github.com/404wolf/valfs/valfs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var valfsConfig = &common.ValfsConfig{}

var fuseCmd = &cobra.Command{
	Short: "Mounting Valfs to a directory",
}

var mountCmd = &cobra.Command{
	Use:   "mount <root>",
	Short: "Mount your Val Town account to a directory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		valfsConfig.MountPoint = args[0]

		// First check direct environment variable
		apiKey := os.Getenv("VAL_TOWN_API_KEY")

		// If not found in environment, try .env file
		if apiKey == "" {
			// Setup Viper for .env
			viper.SetConfigFile(".env")
			viper.SetConfigType("env")
			viper.AutomaticEnv()

			// Read config file
			if err := viper.ReadInConfig(); err != nil {
				// It's okay if there's no config file
				if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
					log.Fatalf("Error reading config file: %v", err)
				}
			}

			// Get API key from Viper
			apiKey = viper.GetString("VAL_TOWN_API_KEY")
		}

		// Final check if API key is set
		if apiKey == "" {
			log.Fatal("VAL_TOWN_API_KEY not found. Please set it in environment or .env file")
		}

		valfsConfig.APIKey = apiKey

		// Create a root node
		root := valfs.NewValFS(
			common.NewClient(
				valfsConfig.APIKey,
				context.Background(),
				valfsConfig.AutoRefresh,
				*valfsConfig,
				logFile,
				verbose,
			),
		)

		if err := root.Mount(func() {
			log.Printf("Mounted ValFS file system from at %s\n", valfsConfig.MountPoint)
		}); err != nil {
			log.Fatalf("Mount failed: %v", err)
		}
	},
}

func ValfsInit() {
	// Add configuration flags specific to valfs (excluding root which is positional)
	mountCmd.Flags().BoolVar(&valfsConfig.DenoCache, "deno-cache", true, "automatically cache required deno packages")
	mountCmd.Flags().BoolVar(&valfsConfig.DenoJson, "deno-json", true, "add a deno.json for editing")
	mountCmd.Flags().BoolVar(&valfsConfig.AutoRefresh, "auto-refresh", true, "automatically refresh content using the api with polling")
	mountCmd.Flags().BoolVar(&valfsConfig.AutoUnmountOnExit, "auto-unmount", true, "automatically unmount directory on exit")
	mountCmd.Flags().IntVar(&valfsConfig.AutoRefreshInterval, "refresh-interval", 300, "how often to poll val town website for changes (in seconds)")
	mountCmd.Flags().BoolVar(&valfsConfig.EnableValsDirectory, "vals-directory", true, "add a directory for your vals")
	mountCmd.Flags().BoolVar(&valfsConfig.GoFuseDebug, "fuse-debug", false, "enable go fuse's debug mode")
	mountCmd.Flags().BoolVar(&valfsConfig.StaticMeta, "static-writes", false, "ensure val file metadata doesn't change on writes")

	// Add commands to the command tree
	fuseCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(fuseCmd)
}
