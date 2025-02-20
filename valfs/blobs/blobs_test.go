package valfs_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	valfs "github.com/404wolf/valfs/valfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dirName = "blobs"

func setupTest(t *testing.T) (*valfs.TestData, string) {
	return valfs.SetupTest(t, dirName)
}

func generateRandomFileName(prefix string) string {
	return fmt.Sprintf("%d%s", rand.Intn(999999), prefix)
}

func TestCreateBlobs(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Create first file", func(t *testing.T) {
		fileName := generateRandomFileName("create1")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("Content of file1"), 0644)
		require.NoError(t, err, "Failed to create file1")
		assert.FileExists(t, filePath, "file1 should exist")
	})

	t.Run("Create second file", func(t *testing.T) {
		fileName := generateRandomFileName("create2")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("Content of file2"), 0644)
		require.NoError(t, err, "Failed to create file2")
		assert.FileExists(t, filePath, "file2 should exist")
	})
}

func TestDeleteBlobs(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Delete first file", func(t *testing.T) {
		fileName := generateRandomFileName("delete1")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("Content to be deleted"), 0644)
		require.NoError(t, err, "Failed to create file")

		_, err = os.Stat(filePath)
		assert.NoError(t, err, "File should exist")

		err = os.Remove(filePath)
		require.NoError(t, err, "Failed to delete file")
		assert.NoFileExists(t, filePath, "Deleted file should not exist")
	})

	t.Run("Delete second file", func(t *testing.T) {
		fileName := generateRandomFileName("delete2")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("Another content to be deleted"), 0644)
		require.NoError(t, err, "Failed to create file")

		err = os.Remove(filePath)
		require.NoError(t, err, "Failed to delete file")
		assert.NoFileExists(t, filePath, "Deleted file should not exist")
	})
}

func TestRenameBlobs(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Rename first file", func(t *testing.T) {
		originalName := generateRandomFileName("original1")
		newName := generateRandomFileName("renamed1")

		originalPath := filepath.Join(blobDir, originalName)
		newPath := filepath.Join(blobDir, newName)

		err := os.WriteFile(originalPath, []byte("Content to be renamed"), 0644)
		require.NoError(t, err, "Failed to create original file")

		err = os.Rename(originalPath, newPath)
		require.NoError(t, err, "Failed to rename file")

		assert.FileExists(t, newPath, "Renamed file should exist")
		assert.NoFileExists(t, originalPath, "Original file should not exist")
	})

	t.Run("Rename second file", func(t *testing.T) {
		originalName := generateRandomFileName("original2")
		newName := generateRandomFileName("renamed2")

		originalPath := filepath.Join(blobDir, originalName)
		newPath := filepath.Join(blobDir, newName)

		err := os.WriteFile(originalPath, []byte("Another content to be renamed"), 0644)
		require.NoError(t, err, "Failed to create original file")

		err = os.Rename(originalPath, newPath)
		require.NoError(t, err, "Failed to rename file from %s to %s", originalPath, newPath)

		assert.FileExists(t, newPath, "Renamed file should exist")
		assert.NoFileExists(t, originalPath, "Original file should not exist")
	})
}

func TestReadBlobs(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Read first file", func(t *testing.T) {
		fileName := generateRandomFileName("read1")
		filePath := filepath.Join(blobDir, fileName)
		content := "Content to be read"

		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err, "Failed to create file")
		time.Sleep(6 * time.Second)

		readContent, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")

		assert.Equal(t, content, string(readContent), "File content should match")
	})

	t.Run("Read second file", func(t *testing.T) {
		fileName := generateRandomFileName("read2")
		filePath := filepath.Join(blobDir, fileName)
		content := "Another content to be read"

		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err, "Failed to create file")
		time.Sleep(6 * time.Second)

		readContent, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")

		assert.Equal(t, content, string(readContent), "File content should match")
	})
}

