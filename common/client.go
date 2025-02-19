package common

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/404wolf/valgo"
)

type Client struct {
	APIClient *APIClient
	APIKey    string
	Config    ValfsConfig
	Id        uint64
	Started   time.Time
	User      valgo.User
}

func NewClient(
	apiKey string,
	ctx context.Context,
	refresh bool,
	config ValfsConfig,
) (*Client, error) {
	apiClientConfig := valgo.NewConfiguration()
	apiClientConfig.AddDefaultHeader(
		"Authorization",
		"Bearer "+apiKey,
	)
	apiClient := NewAPIClient(apiClientConfig)

	userResp, _, err := apiClient.MeAPI.MeGet(ctx).Execute()
	if err != nil {
		return nil, err
	}

	user := valgo.NewUser(
		userResp.Id,
		userResp.Bio,
		userResp.Username,
		userResp.ProfileImageUrl,
		userResp.Url,
		userResp.Links,
	)

	client := &Client{
		APIClient: apiClient,
		APIKey:    apiKey,
		Config:    config,
		Id:        rand.Uint64(),
		Started:   time.Now(),
		User:      *user,
	}

	return client, nil
}
