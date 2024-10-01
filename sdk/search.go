package sdk

import (
	"encoding/json"
	"net/url"
	"time"
)

type Response struct {
	Data  []DataItem `json:"data"`
	Links Links      `json:"links"`
}

type DataItem struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"createdAt"`
	Public    bool      `json:"public"`
	ID        string    `json:"id"`
	Privacy   string    `json:"privacy"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	Author    Author    `json:"author"`
}

type Author struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Links struct {
	Self string `json:"self"`
	Prev string `json:"prev"`
	Next string `json:"next"`
}

type Vals struct {
	Client *ValTownClient
}

func (c *Vals) Search(query string) (*Response, error) {
	const endpoint = "/v1/search/vals"

	fullURL := endpoint + "?query=" + url.QueryEscape(query)

	req, err := c.Client.newRequest("GET", fullURL)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data := &Response{}
	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}
