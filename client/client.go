package client

import (
	"github.com/404wolf/valgo"
)

type Client struct {
	APIClient *valgo.APIClient
}

func NewClient(apiKey string) *Client {
	apiClientConfig := valgo.NewConfiguration()
	apiClientConfig.AddDefaultHeader(
		"Authorization",
		"Bearer "+apiKey,
	)
	apiClient := valgo.NewAPIClient(apiClientConfig)

	return &Client{
		APIClient: apiClient,
	}
}
