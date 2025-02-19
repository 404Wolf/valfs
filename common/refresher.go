package common

import "context"

// Refresher defines the interface for refreshing filesystem contents
type Refresher interface {
	// Refresh updates the filesystem contents
	// ctx provides context for the refresh operation
	// Returns an error if the refresh operation fails
	Refresh(ctx context.Context) error
}

// RefresherConfig holds common configuration for refresher implementations
type RefresherConfig struct {
	// LookupCap determines the maximum number of items to fetch in a single API request
	LookupCap int32
}

// DefaultRefresherConfig returns a RefresherConfig with default values
func DefaultRefresherConfig() RefresherConfig {
	return RefresherConfig{
		LookupCap: 99,
	}
}
