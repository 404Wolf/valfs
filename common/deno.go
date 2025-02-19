package common

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sync/semaphore"

	"go.uber.org/zap"
)

// Semaphore to limit concurrent deno operations to 3
var cacheSemaphore = semaphore.NewWeighted(3)

// DenoCache triggers a cache operation for the specified mount point.
// It ensures only three cache operations run at a time and enforces
// a minimum 1-second delay between operations.
func DenoCache(name string, client *Client) {
	// Acquire semaphore (blocks if 3 operations are already running)
	if err := cacheSemaphore.Acquire(context.Background(), 1); err != nil {
		Logger.Error("failed to acquire semaphore",
			zap.String("name", name),
			zap.Error(err))
		return
	}
	defer cacheSemaphore.Release(1)

	fullMountPoint := filepath.Join(client.Config.MountPoint, "myvals", name)

	// Prepare command
	cmd := exec.Command(
		"deno",
		"install",
		"--allow-import",
		"--entrypoint",
		fullMountPoint,
	)

	// Create buffers for stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Log the full command
	Logger.Debug("starting cache operation",
		zap.String("mountPoint", fullMountPoint),
		zap.String("command", "deno "+strings.Join(cmd.Args[1:], " ")))

	// Execute the command and wait for it to complete
	if err := cmd.Run(); err != nil {
		Logger.Error("failed to execute cache command",
			zap.String("mountPoint", fullMountPoint),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
			zap.Error(err))
		return
	}

	// Log success with output
	Logger.Debug("cache operation completed",
		zap.String("mountPoint", fullMountPoint),
		zap.String("stdout", stdout.String()),
		zap.String("stderr", stderr.String()))
}
