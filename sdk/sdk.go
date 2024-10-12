package sdk

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const BaseURL = "api.val.town"

type ValTownClient struct {
	Bearer     string
	HTTPClient *http.Client
	Vals       Vals
	Me         Me
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
	client.Vals = Vals{}
	client.Vals.Client = client
	client.Me = Me{}
	client.Me.Client = client

	log.Printf("Created new client: %v", client)
	return client, nil
}

func (c *ValTownClient) doRequest(req *http.Request) (*http.Response, error) {
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	logRes(res)
	return res, nil
}

func logReq(req *http.Request) {
	log.Printf("%v", req.Header.Get("Authorization"))
}

func logRes(req *http.Response) {
	log.Printf("%v", req)
}

func (c *ValTownClient) newRequest(method, endpoint string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, "https://"+BaseURL+endpoint, body)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Bearer)
	logReq(req)

	if err != nil {
		log.Fatalf("Error creating request: %v", err)
		return nil, err
	}

	return req, nil
}

// Make a request to the val town API at a specific endpoint with a specific method.
// Include the leading / (e.g. /v1/users) for the endpoint. For POST/PUTs you can include a body.
func (c *ValTownClient) Request(method, endpoint string, body io.Reader) (*http.Response, error) {
	method = strings.ToUpper(method)
	if (method != "POST" && method != "PUT") && body != nil {
		return nil, errors.New("Body is only allowed for POST and PUT requests")
	}

	req, err := c.newRequest(method, endpoint, body)
	logReq(req)
	log.Printf("Request: %v", req)
	if err != nil {
		return nil, err
	}
	return c.doRequest(req)
}
