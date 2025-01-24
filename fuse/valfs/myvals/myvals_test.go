package fuse_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	fuse "github.com/404wolf/valfs/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dirName = "myvals"

func setupTest(t *testing.T) (*fuse.TestData, string) {
	return fuse.SetupTest(t, dirName)
}

func generateRandomFileName(prefix string) string {
	return fmt.Sprintf("val_%d%s", rand.Intn(999999), prefix)
}

func TestCreateFiles(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Create script val", func(t *testing.T) {
		fileName := generateRandomFileName("create1.S.tsx")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("Content of file1"), 0644)
		require.NoError(t, err, "Failed to create file1")
		assert.FileExists(t, filePath, " should exist")
	})

	t.Run("Create http val", func(t *testing.T) {
		fileName := generateRandomFileName("create2.H.tsx")
		filePath := filepath.Join(blobDir, fileName)

		// Create the val
		err := os.WriteFile(filePath, []byte("Content of file2"), 0644)
		require.NoError(t, err, "Failed to create http val")
		assert.FileExists(t, filePath, "http val should exist")

		// Read the val to make sure it has "deployment" in the contents
		contents, err := ioutil.ReadFile(filePath)
		require.NoError(t, err, "Failed to open val file")
		require.Contains(t, string(contents), "deployment:")

		// assert.Contains(t, err
	})

	t.Run("Create invalid name", func(t *testing.T) {
		fileName := generateRandomFileName("create2.H.tsx")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("Content of file2"), 0644)
		require.NoError(t, err, "Failed to create file2")
		assert.FileExists(t, filePath, "file2 should exist")
	})
}
