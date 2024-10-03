package sdk

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
)

const BaseURL = "api.val.town"

type ValTownClient struct {
	Bearer     string
	HTTPClient *http.Client
	Vals       Vals
}

func NewClient() (*ValTownClient, error) {
	apiKey := os.Getenv("VALTOWN_API_KEY")
	if apiKey == "" {
		return nil, errors.New("VALTOWN_API_KEY is not set")
	}
	client := &ValTownClient{
		HTTPClient: &http.Client{},
		Bearer:     apiKey,
	}
	client.Vals = Vals{Client: client}
	log.Printf("Created new client: %v", client)
	return client, nil
}

func (c *ValTownClient) doRequest(req *http.Request) (*http.Response, error) {
	log.Printf("Doing request: %v", req)
	response, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	logReq(response)
	return response, nil
}

func logReq(req *http.Response) {
	req_without_auth := req
	req_without_auth.Header.Del("Authorization")
	log.Println(req_without_auth)
}

func (c *ValTownClient) newRequest(method, endpoint string, body io.Reader) (*http.Request, error) {

	request, err := http.NewRequest(method, "https://"+BaseURL+endpoint, body)

	request.Header.Set("Content-Type", "application/json")
	log.Printf("Created new request: %v", request)
	request.Header.Set("Authorization", "Bearer "+c.Bearer)

	if err != nil {
		log.Fatalf("Error creating request: %v", err)
		return nil, err
	}
	log.Printf("foo")
	return request, nil
}

func (c *ValTownClient) Request(method, endpoint string, body io.Reader) (*http.Response, error) {
	request, err := c.newRequest(method, endpoint, body)
  log.Printf("Request: %v", request)
	if err != nil {
		return nil, err
	}
	return c.doRequest(request)
}
