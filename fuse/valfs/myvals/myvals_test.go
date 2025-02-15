package fuse_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	common "github.com/404wolf/valfs/common"
	fuse "github.com/404wolf/valfs/fuse"
	valfile "github.com/404wolf/valfs/fuse/valfs/myvals/valfile"
	"github.com/404wolf/valgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dirName = "myvals"

func setupTest(t *testing.T) (*fuse.TestData, string) {
	return fuse.SetupTest(t, dirName)
}

func randomFilename(prefix string) string {
	return fmt.Sprintf("val%d%s", rand.Intn(999999), prefix)
}

func getValFromFileContents(t *testing.T, apiClient *common.APIClient, contents string) *valgo.ExtendedVal {
	_, finalMeta, err := valfile.DeconstructVal(string(contents))
	require.NoError(t, err, "Failed to parse final metadata")
	valId := finalMeta.Id
	val, _, err := apiClient.ValsAPI.ValsGet(context.Background(), valId).Execute()
	require.NoError(t, err, "Failed to fetch val")
	return val
}

func TestCreateVal(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Create script val", func(t *testing.T) {
		fileName := randomFilename("create1.S.tsx")
		filePath := filepath.Join(blobDir, fileName)

		// Create the val
		err := os.WriteFile(filePath, []byte("Content of file1"), 0644)
		require.NoError(t, err, "Failed to create file1")
		assert.FileExists(t, filePath, " should exist")

		// Assert the val got created and has correct meta
		finalContents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read final file")
		val := getValFromFileContents(t, testData.APIClient, string(finalContents))
		assert.Equal(t, "script", val.Type, "Type should be 'script'")
	})

	t.Run("Create http val", func(t *testing.T) {
		fileName := randomFilename("create2.H.tsx")
		filePath := filepath.Join(blobDir, fileName)

		// Create the val
		err := os.WriteFile(filePath, []byte("Content of file2"), 0644)
		require.NoError(t, err, "Failed to create http val")
		assert.FileExists(t, filePath, "http val should exist")

		// Assert the val got created and has correct meta
		finalContents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read final file")
		val := getValFromFileContents(t, testData.APIClient, string(finalContents))
		assert.Equal(t, "http", val.Type, "Type should be 'http'")
	})

	t.Run("Create invalid name", func(t *testing.T) {
		fileName := randomFilename("create2.H.tsx")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("Content of file2"), 0644)
		require.NoError(t, err, "Failed to create file2")
		assert.FileExists(t, filePath, "file2 should exist")
	})
}

func TestDeleteVal(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Delete val file", func(t *testing.T) {
		fileName := randomFilename("create3.S.tsx")
		filePath := filepath.Join(blobDir, fileName)

		// Create file first
		err := os.WriteFile(filePath, []byte("Content to delete"), 0644)
		require.NoError(t, err, "Failed to create file for deletion")
		assert.FileExists(t, filePath, "File should exist before deletion")

		// Delete the file
		err = os.Remove(filePath)
		require.NoError(t, err, "Failed to delete file")
		assert.NoFileExists(t, filePath, "File should not exist after deletion")
	})
}

func TestRenameVal(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Rename val file", func(t *testing.T) {
		oldName := randomFilename("original.S.tsx")
		newName := randomFilename("renamed.S.tsx")
		oldPath := filepath.Join(blobDir, oldName)
		newPath := filepath.Join(blobDir, newName)

		// Create initial file
		err := os.WriteFile(oldPath, []byte("Content to rename"), 0644)
		require.NoError(t, err, "Failed to create file for renaming")
		assert.FileExists(t, oldPath, "Original file should exist")

		// Rename the file
		err = os.Rename(oldPath, newPath)
		require.NoError(t, err, "Failed to rename file")

		assert.NoFileExists(t, oldPath, "Original file should not exist")
		assert.FileExists(t, newPath, "Renamed file should exist")
	})
}
