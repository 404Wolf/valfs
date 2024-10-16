package cmd

import (
	"fmt"
	"github.com/404wolf/valfs/sdk"
	"github.com/spf13/cobra"
	"log"
)

var valtCmd = &cobra.Command{
	Use:   "valt",
	Short: "Val town api related actions",
	Args:  cobra.ExactArgs(1),
}

var listValsCommand = &cobra.Command{
	Use:   "list <userId>",
	Short: "List vals for a given user uuid",
	Run: func(cmd *cobra.Command, args []string) {
		userId := args[0]

		client, err := sdk.NewValTownClient()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Listing Vals for", userId)

		vals, _ := client.Vals.OfUser(userId)
		for _, val := range vals {
			fmt.Println(val.Name)
		}
	},
}

var runValCommand = &cobra.Command{
	Use:   "run",
	Short: "Run a val",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := sdk.NewValTownClient()
		if err != nil {
			log.Fatal(err)
		}
		logs, err := client.Vals.Run(args[0])
		fmt.Println(logs)
	},
}

var aboutMeCommand = &cobra.Command{
	Use:   "me",
	Short: "Get information about authed user",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := sdk.NewValTownClient()
		if err != nil {
			log.Fatal(err)
		}
		meInfo, err := client.Me.About()
		if err != nil {
			log.Fatal(err)
		}
		prettyOutput := PrettyPrint(meInfo)
		fmt.Printf("%v", prettyOutput)
	},
}

func ValsInit() {
	rootCmd.AddCommand(valtCmd)
	valtCmd.AddCommand(listValsCommand)
	valtCmd.AddCommand(aboutMeCommand)
}
