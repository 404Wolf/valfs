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

// The top level folder for all val town projects
type ProjectsDir struct {
	fs.Inode

	client   *common.Client
	config   common.RefresherConfig
	stopChan chan struct{}
}

const ProjectsDirPerms = syscall.S_IFDIR | 0555

var _ = (fs.NodeRenamer)((*ProjectsDir)(nil))
var _ = (fs.NodeCreater)((*ProjectsDir)(nil))
var _ = (fs.NodeUnlinker)((*ProjectsDir)(nil))
var _ = (VTFileContainer)((*BranchVTFileContainer)(nil))

var previousProjectIds = make(map[string]*ProjectVTFile)

// A val town project as a folder. All vals, files, and folders in
// the project are subdirectories and the project is the root.
func NewProjectsDir(
	parent *fs.Inode,
	client *common.Client,
	ctx context.Context,
) VTFileContainer {
	common.Logger.Infof("Initializing new ProjectsDir")
	projectDir := &ProjectsDir{
		client:   client,
		config:   common.RefresherConfig{LookupCap: 99},
		stopChan: nil,
	}

	// Add the inode to the parent
	attrs := fs.StableAttr{Mode: ProjectsDirPerms}
	parent.NewPersistentInode(ctx, projectDir, attrs)

	// Initial refresh
	common.Logger.Info("Performing initial refresh of ProjectDir")
	projectDir.Refresh(ctx)

	// Start auto-refresh if configured
	if client.Config.AutoRefresh {
		interval := time.Duration(client.Config.AutoRefreshInterval) * time.Second
		common.Logger.Infof("Starting auto-refresh with interval: %v", interval)
		projectDir.StartAutoRefresh(ctx, interval)
	} else {
		common.Logger.Info("Auto-refresh is disabled by configuration")
	}

	return projectDir
}

func (c *ProjectsDir) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	StartAutoRefreshHelper(ctx, interval, c.Refresh)
}

func (c *ProjectsDir) Refresh(ctx context.Context) error { return nil }

func (c *ProjectsDir) Rename(
	ctx context.Context,
	name string,
	newParent fs.InodeEmbedder,
	newName string,
	flags uint32,
) syscall.Errno {
	return syscall.ENOSYS
}

func (c *ProjectsDir) Create(
	ctx context.Context,
	name string,
	flags uint32,
	mode uint32,
	out *fuse.EntryOut,
) (node *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	return nil, nil, 0, syscall.ENOSYS
}

func (c *ProjectsDir) Unlink(ctx context.Context, name string) syscall.Errno { return syscall.ENOSYS }
func (c *ProjectsDir) GetClient() *common.Client                             { return c.client }
func (c *ProjectsDir) SupportsSubDirs() bool                                 { return false }
func (c *ProjectsDir) GetInode() *fs.Inode                                   { return &c.Inode }
