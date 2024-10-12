package sdk

import (
	"encoding/json"
)

type Me struct {
	Client *ValTownClient
}

type UserProfile struct {
	ID              string     `json:"id"`
	Bio             NullString `json:"bio"`
	Username        NullString `json:"username"`
	ProfileImageURL NullString `json:"profileImageUrl"`
	URL             string     `json:"url"`
	Links           Links      `json:"links"`
	Tier            NullString `json:"tier"`
	Email           string     `json:"email"`
}

type Links struct {
	Self string `json:"self"`
}

type NullString struct {
	String string
	Valid  bool
}

var cachedMe *UserProfile

func (c *Me) About() (*UserProfile, error) {
	if cachedMe != nil {
		return cachedMe, nil
	}

	resp, err := c.Client.Request("GET", "/v1/me", nil)
	if err != nil {
		return nil, err
	}

	data := &UserProfile{}
	err = json.NewDecoder(resp.Body).Decode(data)

	cachedMe = data

	return data, nil
}
