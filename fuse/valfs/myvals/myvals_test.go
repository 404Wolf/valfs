package fuse_test

import (
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

func TestCreateFiles(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Create first file", func(t *testing.T) {
		fileName := fuse.GenerateRandomFileName("create1.S.tsx")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("Content of file1"), 0644)
		require.NoError(t, err, "Failed to create file1")
		assert.FileExists(t, filePath, "file1 should exist")
	})

	t.Run("Create second file", func(t *testing.T) {
		fileName := fuse.GenerateRandomFileName("create2.H.tsx")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("Content of file2"), 0644)
		require.NoError(t, err, "Failed to create file2")
		assert.FileExists(t, filePath, "file2 should exist")
	})
}
