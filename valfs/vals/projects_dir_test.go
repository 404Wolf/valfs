package valfs_test

import (
	"os"
	"path/filepath"
	"testing"

	valfs "github.com/404wolf/valfs/valfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const projectsDirName = "projects"

// Helper functions
func setupProjectsTest(t *testing.T) (*valfs.TestData, string) {
	return valfs.SetupTest(t, projectsDirName)
}

// TestProjectsDirectory tests the basic existence and accessibility of the projects directory
func TestProjectsDirectory(t *testing.T) {
	testData, projectsDir := setupProjectsTest(t)
	defer testData.Cleanup()

	t.Run("Verify projects directory exists", func(t *testing.T) {
		// Check if directory exists
		dirInfo, err := os.Stat(projectsDir)
		require.NoError(t, err, "Projects directory should exist")
		assert.True(t, dirInfo.IsDir(), "Projects path should be a directory")

		// Check if directory is accessible
		_, err = os.ReadDir(projectsDir)
		require.NoError(t, err, "Projects directory should be readable")
	})

	t.Run("Verify projects directory path", func(t *testing.T) {
		expectedPath := filepath.Join(testData.MountPoint, projectsDirName)
		assert.Equal(t, expectedPath, projectsDir, "Projects directory should be in the correct location")
	})
}
