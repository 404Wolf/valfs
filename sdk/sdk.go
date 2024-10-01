package sdk

import (
	"net/http"
	"os"
)

const BaseURL = "api.val.town"

type ValTownClient struct {
	Bearer     string
	HTTPClient *http.Client
	Vals       Vals
}

func NewClient() *ValTownClient {
	client := &ValTownClient{
		HTTPClient: &http.Client{},
		Bearer:     os.Getenv("VALTOWN_API_KEY"),
	}
	client.Vals = Vals{Client: client}
	return client
}

func (c *ValTownClient) doRequest(req *http.Request) (*http.Response, error) {
	return c.HTTPClient.Do(req)
}

func (c *ValTownClient) newRequest(method, endpoint string) (*http.Request, error) {
	req, err := http.NewRequest(method, BaseURL+endpoint, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Bearer)

	if err != nil {
		return nil, err
	}
	return req, nil
}
