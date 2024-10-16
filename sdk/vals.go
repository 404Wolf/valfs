package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"time"
)

type Vals struct {
	Client *ValTownClient
}

type Response struct {
	Data  []ValData `json:"data"`
	Links ValLinks  `json:"links"`
}

type ValData struct {
	Name      string    `json:"name" binding:"required"`
	ID        string    `json:"id" binding:"required,uuid"`
	Version   int       `json:"version" binding:"required,min=0"`
	Code      string    `json:"code" binding:"required"`
	Public    bool      `json:"public" binding:"required"`
	CreatedAt time.Time `json:"createdAt" binding:"required"`
	Privacy   string    `json:"privacy" binding:"required,oneof=public unlisted private"`
	Type      string    `json:"type" binding:"required,oneof=interval http express email script rpc httpnext"`
	URL       string    `json:"url" binding:"required,uri"`
	Links     ValLinks  `json:"links" binding:"required"`
	Author    ValAuthor `json:"author" binding:"required"`
}

// Equal checks if two ValData structs are equal
func (v ValData) Equal(other ValData) bool {
	return v.ID == other.ID
}

// Less checks if this ValData is less than another ValData
func (v ValData) Less(other *ValData) bool {
	return v.ID < other.ID
}

// Hash returns a hash value for the ValData
func (v ValData) Hash() uint64 {
	h := fnv.New64a()
	h.Write([]byte(v.ID))
	return h.Sum64()
}

type ValLinks struct {
	Self     string `json:"self" binding:"required,uri"`
	Versions string `json:"versions" binding:"required,uri"`
	Module   string `json:"module" binding:"required,uri"`
	Endpoint string `json:"endpoint" binding:"omitempty,uri"`
}

type ValUpdate struct {
	ValID   string `json:"val_id" validate:"required"`
	Name    string `json:"name,omitempty" validate:"omitempty,min=1,max=48"`
	Readme  string `json:"readme,omitempty" validate:"omitempty,max=8192"`
	Privacy string `json:"privacy,omitempty" validate:"omitempty,oneof=public unlisted private"`
	Type    string `json:"type,omitempty" validate:"omitempty,oneof=http httpnext script email"`
}

type ValAuthor struct {
	ID       string      `json:"id" binding:"required,uuid"`
	Username interface{} `json:"username"`
}

// Search for a val with a arbitrary textual query
func (c *Vals) Search(query string) ([]ValData, error) {
	fullURL := "/v1/search/vals?query=" + url.QueryEscape(query)

	resp, err := c.Client.Request("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	logRes(resp)

	data := &Response{}
	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		return nil, err
	}

	return data.Data, nil
}

// Fetch a specific val by its ID
func (c *Vals) Fetch(id string) (*ValData, error) {
	resp, err := c.Client.Request("GET", "/v1/vals/"+id, nil)
	if err != nil {
		return nil, err
	}

	data := &ValData{}
	err = json.NewDecoder(resp.Body).Decode(data)

	return data, nil
}

// Update the metadata of a val by its ID
func (c *Vals) Update(id string, data *ValUpdate) error {
	endpoint := "/v1/vals/" + id

	json, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := c.Client.Request("PUT", endpoint, bytes.NewBuffer(json))
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("Failed to update val")
	}
	return nil
}

// Delete a given val by its ID
func (c *Vals) Delete(id string) error {
	endpoint := "/v1/vals/" + id

	resp, err := c.Client.Request("DELETE", endpoint, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("Failed to delete val")
	}
	return nil
}

// Run a val by its ID and log output (not implemented, just prints not implemented)
func (c *Vals) Run(id string) (logs string, err error) {
	fmt.Println("Not implemented")
	return "", nil
}

// List all the vals associated with some specific user
func (c *Vals) OfUser(userId string) ([]ValData, error) {
	resp, err := c.Client.Request("GET", "/v1/users/"+userId+"/vals", nil)
	if err != nil {
		return nil, err
	}

	data := &Response{}
	err = json.NewDecoder(resp.Body).Decode(data)

	return data.Data, nil
}

// List all of the val client's authorized user's vals
func (c *Vals) OfMine() ([]ValData, error) {
	profile, err := c.Client.Me.About()
	userId := profile.ID

	resp, err := c.Client.Request("GET", "/v1/users/"+userId+"/vals", nil)
	if err != nil {
		return nil, err
	}

	data := &Response{}
	err = json.NewDecoder(resp.Body).Decode(data)

	return data.Data, nil
}
