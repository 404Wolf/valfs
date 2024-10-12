package sdk

import (
	"encoding/json"
	"net/url"
	"time"
)

type Response struct {
	Data  []ValData `json:"data"`
	Links ValLinks  `json:"links"`
}

type ValData struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"createdAt"`
	Public    bool      `json:"public"`
	ID        string    `json:"id"`
	Privacy   string    `json:"privacy"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	Author    ValAuthor `json:"author"`
}

type ValAuthor struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type ValLinks struct {
	Self string `json:"self"`
	Prev string `json:"prev"`
	Next string `json:"next"`
}

type Vals struct {
	Client *ValTownClient
}

func (c *Vals) Search(query string) ([]ValData, error) {
	fullURL := "/v1/search/vals?query=" + url.QueryEscape(query)

	resp, err := c.Client.Request("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	logReq(resp)

	data := &Response{}
	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		return nil, err
	}

	return data.Data, nil
}

func (c *Vals) OfUser(userId string) ([]ValData, error) {
	resp, err := c.Client.Request("GET", "/v1/users/"+userId+"/vals", nil)
	if err != nil {
		return nil, err
	}

	data := &Response{}
	err = json.NewDecoder(resp.Body).Decode(data)

	return data.Data, nil
}
