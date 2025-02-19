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

func TestAppendToBlobs(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Append to first file", func(t *testing.T) {
		fileName := generateRandomFileName("append1")
		filePath := filepath.Join(blobDir, fileName)
		initialContent := "Initial content"
		appendedContent := "Appended content"

		err := os.WriteFile(filePath, []byte(initialContent), 0644)
		require.NoError(t, err, "Failed to create file")
		time.Sleep(6 * time.Second)

		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
		require.NoError(t, err, "Failed to open file for appending")
		defer file.Close()

		_, err = file.WriteString(appendedContent)
		require.NoError(t, err, "Failed to append to file")
		time.Sleep(6 * time.Second)

		readContent, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")

		expectedContent := initialContent + appendedContent
		assert.Equal(t, expectedContent, string(readContent), "File content should match")
	})

	t.Run("Append to second file", func(t *testing.T) {
		fileName := generateRandomFileName("append2")
		filePath := filepath.Join(blobDir, fileName)
		initialContent := "Another initial content"
		appendedContent := "Another appended content"

		err := os.WriteFile(filePath, []byte(initialContent), 0644)
		require.NoError(t, err, "Failed to create file")
		time.Sleep(6 * time.Second)

		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
		require.NoError(t, err, "Failed to open file for appending")
		defer file.Close()

		_, err = file.WriteString(appendedContent)
		require.NoError(t, err, "Failed to append to file")
		time.Sleep(6 * time.Second)

		readContent, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")

		expectedContent := initialContent + appendedContent
		assert.Equal(t, expectedContent, string(readContent), "File content should match")
	})

	t.Run("Append same content multiple times", func(t *testing.T) {
		fileName := generateRandomFileName("append_same")
		filePath := filepath.Join(blobDir, fileName)

		content := "aaa\n"
		for i := 0; i < 3; i++ {
			err := appendToFile(filePath, content)
			require.NoError(t, err, "Failed to append to file")
			time.Sleep(6 * time.Second)
		}

		readContent, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")

		// Check if only one instance of the content is present
		assert.Equal(t, "aaa\naaa\naaa\n", string(readContent), "File contents didn't match")
	})

	t.Run("Append different content sequentially", func(t *testing.T) {
		fileName := generateRandomFileName("append_diff")
		filePath := filepath.Join(blobDir, fileName)

		contents := []string{"aaa\n", "bbb\n", "ccc\n"}
		for _, content := range contents {
			err := appendToFile(filePath, content)
			require.NoError(t, err, "Failed to append to file")
			time.Sleep(6 * time.Second)
		}

		readContent, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")

		// Check if only the last appended content is present
		assert.Equal(t, "aaa\nbbb\nccc\n", string(readContent), "File contents didn't match")
	})

	t.Run("Append empty string", func(t *testing.T) {
		fileName := generateRandomFileName("append_empty")
		filePath := filepath.Join(blobDir, fileName)

		initialContent := "Initial content\n"
		err := os.WriteFile(filePath, []byte(initialContent), 0644)
		require.NoError(t, err, "Failed to create file")
		time.Sleep(6 * time.Second)

		err = appendToFile(filePath, "")
		require.NoError(t, err, "Failed to append empty string")
		time.Sleep(6 * time.Second)

		readContent, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")

		assert.Equal(t, initialContent, string(readContent), "File content should remain unchanged after appending empty string")
	})

	t.Run("Append to non-existent file", func(t *testing.T) {
		fileName := generateRandomFileName("append_nonexistent")
		filePath := filepath.Join(blobDir, fileName)

		content := "New content\n"
		err := appendToFile(filePath, content)
		require.NoError(t, err, "Failed to append to non-existent file")
		time.Sleep(6 * time.Second)

		readContent, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")

		assert.Equal(t, content, string(readContent), "File should be created with the appended content")
	})
}

func appendToFile(filePath, content string) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}
