package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/404wolf/valgo"
	"github.com/spf13/cobra"
)

func setupClient() *valgo.APIClient {
	configuration := valgo.NewConfiguration()
	configuration.AddDefaultHeader(
		"Authorization",
		"Bearer "+os.Getenv("VAL_TOWN_API_KEY"),
	)
	apiClient := valgo.NewAPIClient(configuration)
	return apiClient
}

var valtCmd = &cobra.Command{
	Use:   "vals",
	Short: "Val town api related actions",
	Args:  cobra.ExactArgs(1),
}

var listValsCommand = &cobra.Command{
	Use:   "list <userId>",
	Short: "List vals for a given user uuid",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			log.Fatal("Please provide a user id")
		}

		userId := args[0]

		client := setupClient()
		fmt.Println("Listing Vals for", userId)

		req := client.UsersAPI.UsersVals(context.Background(), userId)
		resp, _, err := req.Execute()

		if err != nil {
			log.Fatal(err)
		}

		for _, val := range resp.Data {
			fmt.Println(val.Name)
		}
	},
}

func ValsInit() {
	rootCmd.AddCommand(valtCmd)
	valtCmd.AddCommand(listValsCommand)
}
