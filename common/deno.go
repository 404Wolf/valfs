package common

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// DenoCacher handles caching of files matching glob patterns
type DenoCacher struct {
	cache     map[string][]byte
	timestamp map[string]time.Time
	mutex     sync.RWMutex
}

// NewDenoCacher creates a new instance of DenoCacher
func NewDenoCacher() *DenoCacher {
	return &DenoCacher{
		cache:     make(map[string][]byte),
		timestamp: make(map[string]time.Time),
	}
}

// DenoCache processes files matching the provided glob pattern
func (dc *DenoCacher) DenoCache(glob string) error {
	matches, err := filepath.Glob(glob)
	if err != nil {
		return fmt.Errorf("glob pattern error: %w", err)
	}

	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	for _, match := range matches {
		dc.cache[match] = []byte{}
		dc.timestamp[match] = time.Now()
	}

	return nil
}

// Get retrieves a cached item if it exists
func (dc *DenoCacher) Get(key string) ([]byte, bool) {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	data, exists := dc.cache[key]
	return data, exists
}

// Clear removes all items from the cache
func (dc *DenoCacher) Clear() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.cache = make(map[string][]byte)
	dc.timestamp = make(map[string]time.Time)
}
