package fuse_test

import (
	"os"
	"testing"

	fuse "github.com/404wolf/valfs/fuse"
	"github.com/stretchr/testify/assert"
)

func TestDenoJson(t *testing.T) {
	testData := fuse.SetupTests(t)
	defer testData.Cleanup()

	actualDenoJson, err := os.ReadFile(testData.MountPoint + "/deno.json")
	assert.NoError(t, err)

	expectedDenoJson, err := os.ReadFile("./deno.json")
	assert.NoError(t, err)
	assert.Equal(t, string(expectedDenoJson), string(actualDenoJson))
}
