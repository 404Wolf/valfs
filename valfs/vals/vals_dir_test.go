package valfs_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	common "github.com/404wolf/valfs/common"
	valfs "github.com/404wolf/valfs/valfs"
	vals "github.com/404wolf/valfs/valfs/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dirName = "vals"

func setupTest(t *testing.T) (*valfs.TestData, string) {
	return valfs.SetupTest(t, dirName)
}

func randomFilename(prefix string) string {
	return fmt.Sprintf("val%d%s", rand.Intn(999999), prefix)
}

func getValFromFileContents(contents string, apiClient *common.APIClient) (vals.Val, error) {
	// Extract ID using regex
  pattern := `id:\s*([0-9a-f-]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(contents)

	if len(matches) < 2 {
		return nil, fmt.Errorf("could not find ID in contents")
	}
	id := matches[1]

	// Create a val that we wil fill out with the contents when deserializizing
	val := vals.ValDirValOf(apiClient, id)

	// Create a temporary ValPackage
	tempPackage := vals.NewValPackage(val, false, false)

	// Use UpdateVal to parse the contents - this is the public method for parsing
	err := tempPackage.UpdateVal(contents)
	if err != nil {
		return nil, fmt.Errorf("failed to parse val contents: %w", err)
	}

	// Return the val "stuffed" with what ValPackage plopped into it
	return val, nil
}

func TestBasicValCreation(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Create script val", func(t *testing.T) {
		fileName := randomFilename("create1.S.tsx")
		filePath := filepath.Join(valsDir, fileName)

		// Create the val
		err := os.WriteFile(filePath, []byte("console.log('test');"), 0644)
		require.NoError(t, err, "Failed to create file")
		assert.FileExists(t, filePath, "File should exist")

		// Verify metadata and type
		finalContents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read final file")
		val, err := getValFromFileContents(string(finalContents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		assert.Equal(t, "script", val.GetValType(), "Type should be 'script'")
		assert.Equal(t, int32(0), val.GetVersion(), "Initial version should be 0")
	})

	t.Run("Create http val", func(t *testing.T) {
		fileName := randomFilename("create2.H.tsx")
		filePath := filepath.Join(valsDir, fileName)

		err := os.WriteFile(filePath, []byte("export default function(){return new Response('test')}"), 0644)
		require.NoError(t, err, "Failed to create http val")
		assert.FileExists(t, filePath, "http val should exist")

		finalContents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read final file")
		val, err := getValFromFileContents(string(finalContents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		assert.Equal(t, "http", val.GetValType(), "Type should be 'http'")
	})
}

func TestValPrivacyUpdates(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Privacy setting changes", func(t *testing.T) {
		fileName := randomFilename("privacy.S.tsx")
		filePath := filepath.Join(valsDir, fileName)

		// Create initial val
		_, err := os.Create(filePath)
		require.NoError(t, err, "Failed to create file")

		// Get initial val info
		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")

		// Test each privacy setting
		privacySettings := []string{"public", "private", "unlisted"}
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		for _, privacy := range privacySettings {
			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get val")

			// Update the val's privacy by changing the metadata yaml field
			valPackage := vals.NewValPackage(dirVal, false, false)
			dirVal.SetPrivacy(privacy)
			valText, err := valPackage.ToText()
			require.NoError(t, err, "Failed to serialize val package")
			err = os.WriteFile(filePath, []byte(*valText), 0644)
			require.NoError(t, err, "Failed to write updated val")

			// Verify through API
			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get updated val")
			assert.Equal(t, privacy, dirVal.GetPrivacy(), "Privacy should be "+privacy)
		}
	})
}

func TestValCodeUpdates(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Code updates and versioning", func(t *testing.T) {
		fileName := randomFilename("code.S.tsx")
		filePath := filepath.Join(valsDir, fileName)

		// Create initial val
		initialCode := "console.log('initial');"
		err := os.WriteFile(filePath, []byte(initialCode), 0644)
		require.NoError(t, err, "Failed to create file")

		// Get initial val
		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		// Update code multiple times
		updates := []string{
			"console.log('update 1');",
			"console.log('update 2');",
			"console.log('update 3');",
		}

		for i, newCode := range updates {
			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get val")

			dirVal.SetCode(newCode)
			err = dirVal.Update(ctx)
			require.NoError(t, err, "Failed to update code")

			// Verify version increment and code update
			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get updated val")
			assert.Equal(t, int32(i+1), dirVal.GetVersion(), fmt.Sprintf("Version should be %d", i+1))
			assert.Equal(t, newCode, dirVal.GetCode(), "Code should be updated")
		}
	})
}

func TestValMetadataOperations(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Metadata updates and persistence", func(t *testing.T) {
		fileName := randomFilename("meta.S.tsx")
		filePath := filepath.Join(valsDir, fileName)

		// Create initial val
		err := os.WriteFile(filePath, []byte("console.log('metadata test');"), 0644)
		require.NoError(t, err, "Failed to create file")

		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		err = dirVal.Load(ctx)
		require.NoError(t, err, "Failed to get val")

		// Test readme updates
		newReadme := "Updated readme content"
		dirVal.SetReadme(newReadme)
		err = dirVal.Update(ctx)
		require.NoError(t, err, "Failed to update readme")

		err = dirVal.Load(ctx)
		require.NoError(t, err, "Failed to get updated val")
		assert.Equal(t, newReadme, dirVal.GetReadme(), "Readme should be updated")

		// Verify metadata fields
		assert.NotEmpty(t, dirVal.GetModuleLink(), "Module link should exist")
		assert.NotEmpty(t, dirVal.GetVersionsLink(), "Versions link should exist")
		assert.NotEmpty(t, dirVal.GetAuthorName(), "Author name should exist")
		assert.NotEmpty(t, dirVal.GetAuthorId(), "Author ID should exist")
	})
}

func TestValListingAndDeletion(t *testing.T) {
	testData, _ := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Val listing and deletion", func(t *testing.T) {
		// Get initial count
		initialVals, err := vals.ListValDirVals(ctx, testData.APIClient)
		require.NoError(t, err, "Failed to list initial vals")
		initialCount := len(initialVals)

		// Create new val
		newVal, err := vals.CreateValDirVal(
			ctx,
			testData.APIClient,
			vals.Script,
			"console.log('test');",
			"test_val",
			"unlisted",
		)
		require.NoError(t, err, "Failed to create test val")

		// Verify val is in list
		updatedVals, err := vals.ListValDirVals(ctx, testData.APIClient)
		require.NoError(t, err, "Failed to list updated vals")
		assert.Equal(t, initialCount+1, len(updatedVals), "Should have one more val")

		// Delete val
		err = vals.DeleteValDirVal(ctx, testData.APIClient, newVal.GetId())
		require.NoError(t, err, "Failed to delete val")

		// Verify deletion
		finalVals, err := vals.ListValDirVals(ctx, testData.APIClient)
		require.NoError(t, err, "Failed to list final vals")
		assert.Equal(t, initialCount, len(finalVals), "Should be back to initial count")
	})
}

func TestFileOperations(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Rename val file", func(t *testing.T) {
		oldName := randomFilename("original.S.tsx")
		newName := randomFilename("renamed.S.tsx")
		oldPath := filepath.Join(valsDir, oldName)
		newPath := filepath.Join(valsDir, newName)

		err := os.WriteFile(oldPath, []byte("console.log('rename test');"), 0644)
		require.NoError(t, err, "Failed to create file")
		assert.FileExists(t, oldPath, "Original file should exist")

		err = os.Rename(oldPath, newPath)
		require.NoError(t, err, "Failed to rename file")

		assert.NoFileExists(t, oldPath, "Original file should not exist")
		assert.FileExists(t, newPath, "Renamed file should exist")
	})
}
