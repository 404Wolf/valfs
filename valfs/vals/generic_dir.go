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

// VTFileContainer defines the interface for managing vals in a filesystem context
type VTFileContainer interface {
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
	GetInode() *fs.Inode
	GetClient() *common.Client
	SupportsSubDirs() bool
	Refresh(ctx context.Context) error
	StartAutoRefresh(ctx context.Context, interval time.Duration)
}

// StartAutoRefreshHelper begins automatic refreshing of the vals container
func StartAutoRefreshHelper(
	ctx context.Context,
	interval time.Duration,
	doRefresh func(context.Context) error,
) {
	common.Logger.Infof("Starting auto-refresh with interval %v", interval)
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				common.Logger.Info("Auto-refresh ticker triggered")
				if err := doRefresh(ctx); err != nil {
					common.Logger.Error("Error refreshing vals:", err)
				} else {
					common.Logger.Info("Successfully completed auto-refresh")
				}
			}
		}
	}()
}
