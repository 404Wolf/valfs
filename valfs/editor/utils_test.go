package valfs_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// assertFileExists checks if a file exists at the given path
func assertFileExists(t *testing.T, path string) {
	_, err := os.Stat(path)
	assert.NoError(t, err, "File should exist at path: %s", path)
}

// assertFileContentsMatch compares contents of two files
func assertFileContentsMatch(t *testing.T, expectedPath, actualPath string) {
	expected, err := os.ReadFile(expectedPath)
	assert.NoError(t, err)
	actual, err := os.ReadFile(actualPath)
	assert.NoError(t, err)
	assert.Equal(t, string(expected), string(actual))
}
