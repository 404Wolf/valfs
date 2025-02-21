package valfs_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	common "github.com/404wolf/valfs/common"
	valfs "github.com/404wolf/valfs/valfs"
	vals "github.com/404wolf/valfs/valfs/vals"
	"github.com/404wolf/valgo"
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

func getValFromFileContents(t *testing.T, apiClient *common.APIClient, contents string) *valgo.ExtendedVal {
	_, finalMeta, err := vals.DeconstructVal(string(contents))
	require.NoError(t, err, "Failed to parse final metadata")
	valId := finalMeta.Id
	val, _, err := apiClient.ValsAPI.ValsGet(context.Background(), valId).Execute()
	require.NoError(t, err, "Failed to fetch val")
	return val
}

func TestBasicValCreation(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Create script val", func(t *testing.T) {
		fileName := randomFilename("create1.S.tsx")
		filePath := filepath.Join(blobDir, fileName)

		// Create the val
		err := os.WriteFile(filePath, []byte("console.log('test');"), 0644)
		require.NoError(t, err, "Failed to create file")
		assert.FileExists(t, filePath, "File should exist")

		// Verify metadata and type
		finalContents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read final file")
		val := getValFromFileContents(t, testData.APIClient, string(finalContents))
		assert.Equal(t, "script", val.Type, "Type should be 'script'")
		assert.Equal(t, int32(0), val.Version, "Initial version should be 0")
	})

	t.Run("Create http val", func(t *testing.T) {
		fileName := randomFilename("create2.H.tsx")
		filePath := filepath.Join(blobDir, fileName)

		err := os.WriteFile(filePath, []byte("export default function(){return new Response('test')}"), 0644)
		require.NoError(t, err, "Failed to create http val")
		assert.FileExists(t, filePath, "http val should exist")

		finalContents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read final file")
		val := getValFromFileContents(t, testData.APIClient, string(finalContents))
		assert.Equal(t, "http", val.Type, "Type should be 'http'")
	})
}

func TestValPrivacyUpdates(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Privacy setting changes", func(t *testing.T) {
		fileName := randomFilename("privacy.S.tsx")
		filePath := filepath.Join(blobDir, fileName)

		// Create initial val
		err := os.WriteFile(filePath, []byte("console.log('privacy test');"), 0644)
		require.NoError(t, err, "Failed to create file")

		// Get initial val info
		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val := getValFromFileContents(t, testData.APIClient, string(contents))

		// Test each privacy setting
		privacySettings := []string{"public", "private", "unlisted"}
		dirVal := vals.GetValDirValOf(testData.APIClient, val.Id)

		for _, privacy := range privacySettings {
			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get val")

			dirVal.SetPrivacy(privacy)
			err = dirVal.Update(ctx)
			require.NoError(t, err, "Failed to update privacy to "+privacy)

			// Verify through API
			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get updated val")
			assert.Equal(t, privacy, dirVal.GetPrivacy(), "Privacy should be "+privacy)
		}
	})
}

func TestValCodeUpdates(t *testing.T) {
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Code updates and versioning", func(t *testing.T) {
		fileName := randomFilename("code.S.tsx")
		filePath := filepath.Join(blobDir, fileName)

		// Create initial val
		initialCode := "console.log('initial');"
		err := os.WriteFile(filePath, []byte(initialCode), 0644)
		require.NoError(t, err, "Failed to create file")

		// Get initial val
		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val := getValFromFileContents(t, testData.APIClient, string(contents))
		dirVal := vals.GetValDirValOf(testData.APIClient, val.Id)

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
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Metadata updates and persistence", func(t *testing.T) {
		fileName := randomFilename("meta.S.tsx")
		filePath := filepath.Join(blobDir, fileName)

		// Create initial val
		err := os.WriteFile(filePath, []byte("console.log('metadata test');"), 0644)
		require.NoError(t, err, "Failed to create file")

		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val := getValFromFileContents(t, testData.APIClient, string(contents))
		dirVal := vals.GetValDirValOf(testData.APIClient, val.Id)

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
	testData, blobDir := setupTest(t)
	defer testData.Cleanup()

	t.Run("Rename val file", func(t *testing.T) {
		oldName := randomFilename("original.S.tsx")
		newName := randomFilename("renamed.S.tsx")
		oldPath := filepath.Join(blobDir, oldName)
		newPath := filepath.Join(blobDir, newName)

		err := os.WriteFile(oldPath, []byte("console.log('rename test');"), 0644)
		require.NoError(t, err, "Failed to create file")
		assert.FileExists(t, oldPath, "Original file should exist")

		err = os.Rename(oldPath, newPath)
		require.NoError(t, err, "Failed to rename file")

		assert.NoFileExists(t, oldPath, "Original file should not exist")
		assert.FileExists(t, newPath, "Renamed file should exist")
	})
}
