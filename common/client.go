package common

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/404wolf/valgo"
	"go.uber.org/zap"
)

type Client struct {
	APIClient  *APIClient
	APIKey     string
	Config     ValfsConfig
	DenoCacher *DenoCacher
	Id         uint64
	Logger     *zap.SugaredLogger
	Started    time.Time
	User       valgo.User
}

func NewClient(
	apiKey string,
	ctx context.Context,
	refresh bool,
	config ValfsConfig,
	logFile string,
	verbose bool,
) *Client {
	apiClientConfig := valgo.NewConfiguration()
	apiClientConfig.AddDefaultHeader(
		"Authorization",
		"Bearer "+apiKey,
	)
	apiClient := NewAPIClient(apiClientConfig)

	userResp, _, _ := apiClient.MeAPI.MeGet(ctx).Execute()
	user := valgo.NewUser(
		userResp.Id,
		userResp.Bio,
		userResp.Username,
		userResp.ProfileImageUrl,
		userResp.Url,
		userResp.Links,
	)

	client := &Client{
		APIClient:  apiClient,
		APIKey:     apiKey,
		Config:     config,
		DenoCacher: NewDenoCacher(),
		Id:         rand.Uint64(),
		Started:    time.Now(),
		User:       *user,
	}

	// Setup logger after client is initialized
	client.Logger = client.setupLogger(logFile, verbose)

	return client
}
