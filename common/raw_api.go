package common

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/404wolf/valgo"
)

type APIClient struct {
	*valgo.APIClient
	cfg *valgo.Configuration
}

func NewAPIClient(cfg *valgo.Configuration) *APIClient {
	return &APIClient{
		APIClient: valgo.NewAPIClient(cfg),
		cfg:       cfg,
	}
}

func (c *APIClient) RawRequest(
	ctx context.Context,
	method, path string,
	body io.Reader,
) (*http.Response, error) {
	// Use the first server URL if available, otherwise fall back to Scheme and Host
	var baseURL string
	if len(c.cfg.Servers) > 0 {
		baseURL = c.cfg.Servers[0].URL
	} else {
		baseURL = c.cfg.Scheme + "://" + c.cfg.Host
	}

	// Construct the full URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}

	// Add default headers
	for k, v := range c.cfg.DefaultHeader {
		req.Header.Add(k, v)
	}

	// Set User-Agent
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}

	// Use the configured HTTP client
	client := c.cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	// Send the request
	return client.Do(req)
}

