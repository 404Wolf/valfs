package valfs

import (
	"context"
	"os/exec"
	"sync"
	"time"

	common "github.com/404wolf/valfs/common"
)

// DenoCacher handles caching of files matching glob patterns
type DenoCacher struct {
	client    *common.Client
	cache     map[string][]byte
	timestamp map[string]time.Time
	mutex     sync.RWMutex
	context   context.Context
}

// NewDenoCacher creates a new instance of DenoCacher
func NewDenoCacher() *DenoCacher {
	return &DenoCacher{
		cache:     make(map[string][]byte),
		timestamp: make(map[string]time.Time),
		context:   context.Background(),
	}
}

// DenoCache processes files matching the provided glob pattern
func (dc *DenoCacher) DenoCache(glob string) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for range ticker.C {
			cmd := exec.Command(
				"deno",
				"cache",
				"--allow-import",
				dc.client.Config.MountPoint+"/myvals",
			)
			cmd.Start()
			cmd.Process.Release()
		}
	}()
}
