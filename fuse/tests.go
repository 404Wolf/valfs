package fuse

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

// TestData holds information about the test environment
type TestData struct {
	MountPoint  string
	Cleanup     func()
	Unmount     func()
	Mount       func()
	cmd         *exec.Cmd
	projectRoot string
}

// findProjectRoot locates the root directory of the project by searching for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// setupTest prepares a new test environment for each test
func SetupTest(t *testing.T, dirName string) (*TestData, string) {
	t.Helper()

	// Set up the test filesystem
	testData := SetupTests(t)

	// Define the blob directory within the mount point
	blobDir := filepath.Join(testData.MountPoint, dirName)

	return &testData, blobDir
}

// SetupTests prepares the test environment and returns necessary data
func SetupTests(t *testing.T) TestData {
	t.Helper()

	// Find the project root directory
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("Failed to find project root: %v", err)
	}

	// Load environment variables from .env file
	err = godotenv.Load(filepath.Join(projectRoot, ".env"))
	if err != nil {
		// If .env file is not found, log a warning but continue
		t.Logf("Warning: Error loading .env file: %v", err)
		t.Log("Falling back to system environment variables")
	}

	// Check for required environment variables
	requiredEnvVars := []string{"VAL_TOWN_API_KEY"} // Add other required variables here
	for _, envVar := range requiredEnvVars {
		if value := os.Getenv(envVar); value == "" {
			t.Fatalf("Required environment variable %s is not set", envVar)
		}
	}
	// Create a temporary directory for the test
	testDir, err := os.MkdirTemp("", "valfs-tests-")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	// Path to the valfs binary
	valfsPath := filepath.Join(projectRoot, "valfs")

	var cmd *exec.Cmd

	// Mount the valfs file system
	mount := func() {
		cmd = exec.Command(valfsPath, "mount", testDir, "--verbose", "--no-refresh")
		cmd.Env = os.Environ()
		cmd.Dir = projectRoot

		// Create a pipe for stdout
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("Failed to create stdout pipe: %v", err)
		}

		// Start the command
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start valfs mount command: %v", err)
		}

		// Start a goroutine to read from the pipe and print to console
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				fmt.Println(scanner.Text())
			}
		}()

		// Wait for the filesystem to be mounted
		if !waitForMount(testDir) {
			t.Fatalf("Filesystem did not mount within the expected time")
		}
	}

	// Unmount the valfs file system
	unmount := func() {
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Signal(os.Interrupt)
			cmd.Wait()
		}
	}

	// Initial mount
	mount()

	// Prepare cleanup function
	cleanup := func() {
		unmount()
		os.RemoveAll(testDir + "/myblobs")
		os.RemoveAll(testDir + "/myvals")
	}

	return TestData{
		MountPoint:  testDir,
		Cleanup:     cleanup,
		Unmount:     unmount,
		Mount:       mount,
		cmd:         cmd,
		projectRoot: projectRoot,
	}
}

// waitForMount checks if the filesystem is mounted by looking for deno.json
func waitForMount(dir string) bool {
	for i := 0; i < 90; i++ {
		time.Sleep(250 * time.Millisecond)
		if _, err := os.Stat(filepath.Join(dir, "deno.json")); err == nil {
			return true
		}
	}
	return false
}
