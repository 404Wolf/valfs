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

// The folder with all of your val town projects in it
type ProjectsDir struct {
	ValsDir // Embed ValsDir to inherit its functionality
}

// Set up background refresh of val town projects and retrieve an auto updating
// folder of val projects
func NewProjectsDir(
	parent *fs.Inode,
	client *common.Client,
	ctx context.Context,
) ValsContainer {
	common.Logger.Info("Initializing new ProjectsDir")
	projectsDir := &ProjectsDir{
		ValsDir: ValsDir{
			client:   client,
			config:   common.RefresherConfig{LookupCap: 99},
			stopChan: nil,
		},
	}

	// Add the inode to the parent
	attrs := fs.StableAttr{Mode: syscall.S_IFDIR | 0555}
	parent.NewPersistentInode(ctx, projectsDir, attrs)

	// Initial refresh
	common.Logger.Info("Performing initial refresh of ProjectsDir")
	projectsDir.Refresh(ctx)

	// Start auto-refresh if configured
	if client.Config.AutoRefresh {
		interval := time.Duration(client.Config.AutoRefreshInterval) * time.Second
		common.Logger.Infof("Starting auto-refresh with interval: %v", interval)
		projectsDir.StartAutoRefresh(ctx, interval)
	} else {
		common.Logger.Info("Auto-refresh is disabled by configuration")
	}

	return projectsDir
}

// Override SupportsDirs to return true for projects
func (c *ProjectsDir) SupportsDirs() bool {
	return true
}

// Remove FUSE operations by setting them to nil
func (c *ProjectsDir) Create(
	ctx context.Context,
	name string,
	flags uint32,
	mode uint32,
	out *fuse.EntryOut,
) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	return nil, nil, 0, syscall.ENOSYS
}

func (c *ProjectsDir) Unlink(ctx context.Context, name string) syscall.Errno {
	return syscall.ENOSYS
}

func (c *ProjectsDir) Rename(
	ctx context.Context,
	oldName string,
	newParent fs.InodeEmbedder,
	newName string,
	flags uint32,
) syscall.Errno {
	return syscall.ENOSYS
}
