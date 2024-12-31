package client

import (
	"context"
	"log"

	"github.com/404wolf/valgo"
)

type Client struct {
	User      valgo.User
	APIClient *valgo.APIClient
}

func NewClient(apiKey string, ctx context.Context) *Client {
	log.Printf("Creating new client")

	apiClientConfig := valgo.NewConfiguration()
	apiClientConfig.AddDefaultHeader(
		"Authorization",
		"Bearer "+apiKey,
	)
	apiClient := valgo.NewAPIClient(apiClientConfig)
	log.Printf("API client: %v", apiClient)

	userResp, _, err := apiClient.MeAPI.MeGet(ctx).Execute()
	if err != nil {
		log.Fatalf(err.Error())
	}
	user := valgo.NewUser(
		userResp.Id,
		userResp.Bio,
		userResp.Username,
		userResp.ProfileImageUrl,
		userResp.Url,
		userResp.Links,
	)
	log.Printf("Active user: %v", user.GetUsername())

	client := &Client{
		APIClient: apiClient,
		User:      *user,
	}

	log.Printf("Client: %v", client)
	return client
}
