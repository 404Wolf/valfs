package valfs

import (
	"context"

	"github.com/404wolf/valgo"
)

// ValOperations defines the interface for all val-related operations
type ValOperations interface {
	// Create creates a new val with the given parameters
	Create(ctx context.Context, name, valType, code, privacy string) (*valgo.ExtendedVal, error)

	// Read retrieves a val's content by its ID
	Read(ctx context.Context, valId string) (*valgo.ExtendedVal, error)

	// Update updates an existing val's properties
	Update(ctx context.Context, valId string, name, valType string) (*valgo.ExtendedVal, error)

	// UpdateCode updates a val's code content
	UpdateCode(ctx context.Context, valId string, code string) error

	// Delete removes a val by its ID
	Delete(ctx context.Context, valId string) error

	// List retrieves all vals for the authenticated user
	List(ctx context.Context, offset, limit int32) ([]valgo.BasicVal, error)
}
