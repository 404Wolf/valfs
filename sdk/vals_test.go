package sdk

import (
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func getTestClient() (*ValTownClient, error) {
	return NewValTownClient()
}

func TestMain(t *testing.M) {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	code := t.Run()
	os.Exit(code)
}

func TestNewVal(t *testing.T) {
	client, err := getTestClient()
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
	client, err := getTestClient()
	assert.NoError(t, err, "Error creating new client")

	_, err = client.Vals.Search("test")
	assert.NoError(t, err, "Error searching")
}

func TestMyVals(t *testing.T) {
	client, err := getTestClient()
	assert.NoError(t, err, "Error creating new client")

	result, err := client.Vals.OfMine()
	assert.NoError(t, err, "Error fetching my vals")
	assert.Greater(t, len(result), 0, "No vals found")
}
