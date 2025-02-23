package common

import "context"

// Refresher defines the interface for refreshing filesystem contents
type Refresher interface {
	// Refresh updates the filesystem contents
	// ctx provides context for the refresh operation
	// Returns an error if the refresh operation fails
	Refresh(ctx context.Context) error
}
