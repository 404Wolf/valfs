package common

import (
	"context"
	"log"
	"math/rand/v2"
	"time"

	"github.com/404wolf/valgo"
	"go.uber.org/zap"
)

type Client struct {
	User       valgo.User
	Logger     zap.Logger
	APIKey     string
	APIClient  *APIClient
	Config     ValfsConfig
	Started    time.Time
	Id         uint64
	DenoCacher *DenoCacher
}

func NewClient(apiKey string, ctx context.Context, refresh bool, config ValfsConfig) *Client {
	log.Printf("Creating new client")

	apiClientConfig := valgo.NewConfiguration()
	apiClientConfig.AddDefaultHeader(
		"Authorization",
		"Bearer "+apiKey,
	)
	apiClient := NewAPIClient(apiClientConfig)
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
		APIClient:  apiClient,
		APIKey:     apiKey,
		Config:     config,
		DenoCacher: NewDenoCacher(),
		User:       *user,
		Started:    time.Now(),
		Id:         rand.Uint64(),
	}

	log.Printf("Client: %v", client)
	return client
}
