package sdk

import (
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func TestMain(t *testing.M) {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	code := t.Run()
	os.Exit(code)
}

func TestNewVal(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Errorf("Error creating new client: %v", err)
	}
	if client == nil {
		t.Errorf("Client is nil")
	}
	if client.Vals.Client != client {
		t.Errorf("Client is not set in Vals")
	}
}

func TestValsSearch(t *testing.T) {
	client, err := NewClient()
	assert.NoError(t, err, "Error creating new client")

	resp, err := client.Vals.Search("test")
	assert.NoError(t, err, "Error searching")
	assert.NotNil(t, resp.Data, "Data is nil")
	assert.NotEmpty(t, resp.Data, "Data is empty")
	assert.NotEmpty(t, resp.Data[0].Name, "Name is not test")
}
