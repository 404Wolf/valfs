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

// Helper functions
func setupTest(t *testing.T) (*valfs.TestData, string) {
	return valfs.SetupTest(t, dirName)
}

func randomFilename(prefix string) string {
	return fmt.Sprintf("val%d%s", rand.Intn(999999), prefix)
}

func getValFromFileContents(contents string, apiClient *common.APIClient) (vals.Val, error) {
	pattern := `id:\s*([0-9a-f-]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(contents)

	if len(matches) < 2 {
		return nil, fmt.Errorf("could not find ID in contents")
	}
	id := matches[1]

	val := vals.ValDirValOf(apiClient, id)
	tempPackage := vals.NewValPackage(val, false, false)

	err := tempPackage.UpdateVal(contents)
	if err != nil {
		return nil, fmt.Errorf("failed to parse val contents: %w", err)
	}

	return val, nil
}

// TestValCreation tests the creation of different types of vals
func TestValCreation(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Create script val", func(t *testing.T) {
		fileName := randomFilename("create1.S.tsx")
		filePath := filepath.Join(valsDir, fileName)

		_, err := os.Create(filePath)
		require.NoError(t, err, "Failed to create file")

		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		valPackage := vals.NewValPackage(dirVal, false, false)
		dirVal.SetCode("console.log('test');")
		valText, err := valPackage.ToText()
		require.NoError(t, err, "Failed to serialize val package")
		err = os.WriteFile(filePath, []byte(*valText), 0644)
		require.NoError(t, err, "Failed to write val")
		assert.FileExists(t, filePath, "File should exist")

		err = dirVal.Load(ctx)
		require.NoError(t, err, "Failed to get val from API")
		assert.Equal(t, vals.Script, dirVal.GetValType(), "Type should be 'script'")
		assert.Equal(t, int32(1), dirVal.GetVersion(), "Initial version should be 1 after first write")
		assert.Equal(t, "console.log('test');", dirVal.GetCode(), "Code should match")
	})

	t.Run("Create http val", func(t *testing.T) {
		fileName := randomFilename("create2.H.tsx")
		filePath := filepath.Join(valsDir, fileName)

		_, err := os.Create(filePath)
		require.NoError(t, err, "Failed to create file")

		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		valPackage := vals.NewValPackage(dirVal, false, false)
		dirVal.SetCode("export default function(){return new Response('test')}")
		valText, err := valPackage.ToText()
		require.NoError(t, err, "Failed to serialize val package")
		err = os.WriteFile(filePath, []byte(*valText), 0644)
		require.NoError(t, err, "Failed to write val")
		assert.FileExists(t, filePath, "http val should exist")

		err = dirVal.Load(ctx)
		require.NoError(t, err, "Failed to get val from API")
		assert.Equal(t, vals.HTTP, dirVal.GetValType(), "Type should be 'http'")
		assert.Equal(t, "export default function(){return new Response('test')}", dirVal.GetCode(), "Code should match")
	})
}

// TestValUpdates tests various update operations on vals
func TestValUpdates(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Privacy setting updates", func(t *testing.T) {
		fileName := randomFilename("privacy.S.tsx")
		filePath := filepath.Join(valsDir, fileName)

		_, err := os.Create(filePath)
		require.NoError(t, err, "Failed to create file")

		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")

		privacySettings := []string{"public", "private", "unlisted"}
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		for _, privacy := range privacySettings {
			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get val")

			valPackage := vals.NewValPackage(dirVal, false, false)
			dirVal.SetPrivacy(privacy)
			valText, err := valPackage.ToText()
			require.NoError(t, err, "Failed to serialize val package")
			err = os.WriteFile(filePath, []byte(*valText), 0644)
			require.NoError(t, err, "Failed to write updated val")

			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get updated val")
			assert.Equal(t, privacy, dirVal.GetPrivacy(), "Privacy should be "+privacy)
		}
	})

	t.Run("Code updates and versioning", func(t *testing.T) {
		fileName := randomFilename("code.S.tsx")
		filePath := filepath.Join(valsDir, fileName)

		_, err := os.Create(filePath)
		require.NoError(t, err, "Failed to create file")

		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		updates := []string{
			"console.log('update 1');",
			"console.log('update 2');",
			"console.log('update 3');",
		}

		for i, newCode := range updates {
			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get val")

			valPackage := vals.NewValPackage(dirVal, false, false)
			dirVal.SetCode(newCode)
			valText, err := valPackage.ToText()
			require.NoError(t, err, "Failed to serialize val package")
			err = os.WriteFile(filePath, []byte(*valText), 0644)
			require.NoError(t, err, "Failed to write updated val")

			err = dirVal.Load(ctx)
			require.NoError(t, err, "Failed to get updated val")
			assert.Equal(t, int32(i+1), dirVal.GetVersion(), fmt.Sprintf("Version should be %d", i+1))
			assert.Equal(t, newCode, dirVal.GetCode(), "Code should be updated")
		}
	})
}

// TestValMetadata tests metadata-related operations
func TestValMetadata(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Metadata updates and persistence", func(t *testing.T) {
		fileName := randomFilename("meta.S.tsx")
		filePath := filepath.Join(valsDir, fileName)

		_, err := os.Create(filePath)
		require.NoError(t, err, "Failed to create file")

		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		err = dirVal.Load(ctx)
		require.NoError(t, err, "Failed to get val")

		newReadme := "Updated readme content"
		valPackage := vals.NewValPackage(dirVal, false, false)
		dirVal.SetReadme(newReadme)
		valText, err := valPackage.ToText()
		require.NoError(t, err, "Failed to serialize val package")
		err = os.WriteFile(filePath, []byte(*valText), 0644)
		require.NoError(t, err, "Failed to write updated val")

		err = dirVal.Load(ctx)
		require.NoError(t, err, "Failed to get updated val")
		assert.Equal(t, newReadme, dirVal.GetReadme(), "Readme should be updated")

		assert.NotEmpty(t, dirVal.GetModuleLink(), "Module link should exist")
		assert.NotEmpty(t, dirVal.GetVersionsLink(), "Versions link should exist")
		assert.NotEmpty(t, dirVal.GetAuthorName(), "Author name should exist")
		assert.NotEmpty(t, dirVal.GetAuthorId(), "Author ID should exist")
	})
}

// TestValManagement tests listing and deletion operations
func TestValManagement(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Val listing and deletion", func(t *testing.T) {
		initialVals, err := vals.ListValDirVals(ctx, testData.APIClient)
		require.NoError(t, err, "Failed to list initial vals")
		initialCount := len(initialVals)

		fileName := randomFilename("listing.S.tsx")
		filePath := filepath.Join(valsDir, fileName)

		_, err = os.Create(filePath)
		require.NoError(t, err, "Failed to create file")

		contents, err := os.ReadFile(filePath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		valPackage := vals.NewValPackage(dirVal, false, false)
		dirVal.SetCode("console.log('test');")
		valText, err := valPackage.ToText()
		require.NoError(t, err, "Failed to serialize val package")
		err = os.WriteFile(filePath, []byte(*valText), 0644)
		require.NoError(t, err, "Failed to write val")

		updatedVals, err := vals.ListValDirVals(ctx, testData.APIClient)
		require.NoError(t, err, "Failed to list updated vals")
		assert.Equal(t, initialCount+1, len(updatedVals), "Should have one more val")

		err = os.Remove(filePath)
		require.NoError(t, err, "Failed to delete val file")

		finalVals, err := vals.ListValDirVals(ctx, testData.APIClient)
		require.NoError(t, err, "Failed to list final vals")
		assert.Equal(t, initialCount, len(finalVals), "Should be back to initial count")
	})
}

// TestFileOperations tests file system operations
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

func TestValTypeChange(t *testing.T) {
	testData, valsDir := setupTest(t)
	defer testData.Cleanup()

	ctx := context.Background()

	t.Run("Change val type while preserving code", func(t *testing.T) {
		// Start with a script val
		scriptName := randomFilename("typechange.S.tsx")
		scriptPath := filepath.Join(valsDir, scriptName)

		_, err := os.Create(scriptPath)
		require.NoError(t, err, "Failed to create initial script file")

		contents, err := os.ReadFile(scriptPath)
		require.NoError(t, err, "Failed to read file")
		val, err := getValFromFileContents(string(contents), testData.APIClient)
		require.NoError(t, err, "Failed to get val from file contents")
		dirVal := vals.ValDirValOf(testData.APIClient, val.GetId())

		// Set initial code
		testCode := "export default function handler(req) { return new Response('Hello!'); }"
		valPackage := vals.NewValPackage(dirVal, false, false)
		dirVal.SetCode(testCode)
		valText, err := valPackage.ToText()
		require.NoError(t, err, "Failed to serialize val package")
		err = os.WriteFile(scriptPath, []byte(*valText), 0644)
		require.NoError(t, err, "Failed to write val")

		// Change to HTTP val by renaming
		httpName := randomFilename("typechange.H.tsx")
		httpPath := filepath.Join(valsDir, httpName)

		err = os.Rename(scriptPath, httpPath)
		require.NoError(t, err, "Failed to rename file")

		// Verify code persisted after type change
		err = dirVal.Load(ctx)
		require.NoError(t, err, "Failed to load val after type change")
		assert.Equal(t, vals.HTTP, dirVal.GetValType(), "Type should be changed to HTTP")
		assert.Equal(t, testCode, dirVal.GetCode(), "Code should remain unchanged after type change")

		// Read the file contents and verify deployment URL exists
		httpContents, err := os.ReadFile(httpPath)
		require.NoError(t, err, "Failed to read HTTP val file")
		deploymentURLPattern := regexp.MustCompile(`deployment:\s*https://.*\.web\.val\.run`)
		assert.True(t, deploymentURLPattern.Match(httpContents), "HTTP val should have deployment URL")

		// Change back to Script val
		scriptName = randomFilename("typechange.S.tsx")
		scriptPath = filepath.Join(valsDir, scriptName)
		err = os.Rename(httpPath, scriptPath)
		require.NoError(t, err, "Failed to rename back to script")

		// Read the file contents and verify deployment URL is removed
		scriptContents, err := os.ReadFile(scriptPath)
		require.NoError(t, err, "Failed to read Script val file")
		assert.False(t, deploymentURLPattern.Match(scriptContents), "Script val should not have deployment URL")

		// Verify code still persists
		err = dirVal.Load(ctx)
		require.NoError(t, err, "Failed to load val after changing back to script")
		assert.Equal(t, vals.Script, dirVal.GetValType(), "Type should be changed to Script")
		assert.Equal(t, testCode, dirVal.GetCode(), "Code should remain unchanged after changing back to script")
	})
}
