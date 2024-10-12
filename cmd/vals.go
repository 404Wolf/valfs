package cmd

import (
	"fmt"
	"github.com/404wolf/valfs/sdk"
	"github.com/spf13/cobra"
	"log"
)

var valsCmd = &cobra.Command{
	Use:   "vals [run]",
	Short: "Val related actions",
	Args:  cobra.ExactArgs(1),
}

var listValsCommand = &cobra.Command{
	Use:   "list <userId>",
	Short: "List vals for a given user uuid",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		userId := args[0]

		client, err := sdk.NewClient()
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

func ValsInit() {
	rootCmd.AddCommand(valsCmd)
	valsCmd.AddCommand(listValsCommand)
}
