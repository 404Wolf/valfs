package valfs_test

import (
	"path/filepath"
	"testing"

	valfs "github.com/404wolf/valfs/valfs"
)

func TestDenoJson(t *testing.T) {
	testData := valfs.SetupTests(t)
	defer testData.Cleanup()
	assertFileContentsMatch(
		t,
		"deno.json",
		filepath.Join(testData.MountPoint, "/deno.json"),
	)
}
