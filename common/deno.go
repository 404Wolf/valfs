package common

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	lastCacheTime = time.Now()
	cacheMutex    sync.Mutex
)

// DenoCache triggers a cache operation for the specified mount point.
// It ensures only one cache operation runs at a time and enforces
// a minimum 1-second delay between operations.
func DenoCache(name string, client *Client) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	fullMountPoint := filepath.Join(client.Config.MountPoint, "myvals", name)

	// Check if enough time has passed since the last cache
	if time.Since(lastCacheTime) < time.Second {
		Logger.Debug("skipping cache operation: minimum delay not met",
			zap.String("mountPoint", name),
			zap.Duration("timeSinceLastCache", time.Since(lastCacheTime)))
		return
	}

	// Update last cache time
	lastCacheTime = time.Now()

	// Prepare command
	cmd := exec.Command(
		"deno",
		"cache",
		"--allow-import",
		fullMountPoint,
	)

	// Log the full command
	Logger.Debug("starting cache operation",
		zap.String("mountPoint", fullMountPoint),
		zap.String("command", "deno "+strings.Join(cmd.Args[1:], " ")))

	// Execute the cache command
	if err := cmd.Start(); err != nil {
		Logger.Error("failed to start cache command",
			zap.String("mountPoint", fullMountPoint),
			zap.Error(err))
		return
	}
	if err := cmd.Process.Release(); err != nil {
		Logger.Error("failed to release cache process",
			zap.String("mountPoint", fullMountPoint),
			zap.Error(err))
	}

	Logger.Debug("cache operation initiated",
		zap.String("mountPoint", fullMountPoint))
}
