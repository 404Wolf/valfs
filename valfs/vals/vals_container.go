package valfs

import (
	"context"
	"syscall"
	"time"

	_ "embed"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	common "github.com/404wolf/valfs/common"
)

// ValsContainer defines the interface for managing vals in a filesystem context
type ValsContainer interface {
	// Core filesystem operations
	Create(
		ctx context.Context,
		name string,
		flags uint32,
		mode uint32,
		entryOut *fuse.EntryOut,
	) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, code syscall.Errno)
	Unlink(ctx context.Context, name string) syscall.Errno
	Rename(
		ctx context.Context,
		oldName string,
		newParent fs.InodeEmbedder,
		newName string,
		flags uint32,
	) syscall.Errno

	// Val management
	GetChild(name string) *fs.Inode
	GetInode() *fs.Inode
	NewPersistentInode(ctx context.Context, ops fs.InodeEmbedder, attr fs.StableAttr) *fs.Inode

	// Client access
	GetClient() *common.Client

	// Directory capabilities
	IsRoot() bool       // Whether this is the root vals container
	SupportsDirs() bool // Whether this container supports subdirectories

	// Refresh capabilities
	Refresh(ctx context.Context) error
	StartAutoRefresh(ctx context.Context, interval time.Duration)
	StopAutoRefresh()
}
