package valfs

import (
	"context"
	"syscall"
	"time"

	_ "embed"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
)

// The folder for a specific branch of a project
type BranchVTFileContainer struct {
	fs.Inode
	Branch *valgo.Branch

	client   *common.Client
	config   common.RefresherConfig
	stopChan chan struct{}
}

const BranchDirPerms = syscall.S_IFDIR | 0755

var _ = (fs.NodeRenamer)((*BranchVTFileContainer)(nil))
var _ = (fs.NodeCreater)((*BranchVTFileContainer)(nil))
var _ = (fs.NodeUnlinker)((*BranchVTFileContainer)(nil))
var _ = (VTFileContainer)((*BranchVTFileContainer)(nil))

var previousBranchIds = make(map[string]*RegularValVTFile)

// A branch of a val town project, as a folder. All vals, files, and folders in
// the branch are subdirectories and the branch is the root.
func NewBranchesDir(
	parent *fs.Inode,
	client *common.Client,
	branch *valgo.Branch,
	ctx context.Context,
) VTFileContainer {
	common.Logger.Info("Initializing new BranchesDir")
	branchDir := &BranchVTFileContainer{
		Branch:   branch,
		client:   client,
		config:   common.RefresherConfig{LookupCap: 99},
		stopChan: nil,
	}

	// Add the inode to the parent
	attrs := fs.StableAttr{Mode: BranchDirPerms}
	parent.NewPersistentInode(ctx, branchDir, attrs)

	// Initial refresh
	common.Logger.Info("Performing initial refresh of BranchDir")
	branchDir.Refresh(ctx)

	// Start auto-refresh if configured
	if client.Config.AutoRefresh {
		interval := time.Duration(client.Config.AutoRefreshInterval) * time.Second
		common.Logger.Infof("Starting auto-refresh with interval: %v", interval)
		branchDir.StartAutoRefresh(ctx, interval)
	} else {
		common.Logger.Info("Auto-refresh is disabled by configuration")
	}

	return branchDir
}

func (c *BranchVTFileContainer) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	common.Logger.Info("BranchDir auto-refresh start stub")
}
func (c *BranchVTFileContainer) Refresh(ctx context.Context) error {
	common.Logger.Info("BranchDir refresh stub")
	return nil
}
func (c *BranchVTFileContainer) Rename(
	ctx context.Context,
	name string,
	newParent fs.InodeEmbedder,
	newName string,
	flags uint32,
) syscall.Errno {
	return syscall.ENOSYS
}
func (c *BranchVTFileContainer) Create(
	ctx context.Context,
	name string,
	flags uint32,
	mode uint32,
	out *fuse.EntryOut,
) (node *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	return nil, nil, 0, syscall.ENOSYS
}
func (c *BranchVTFileContainer) Unlink(ctx context.Context, name string) syscall.Errno {
	return syscall.ENOSYS
}
func (c *BranchVTFileContainer) GetClient() *common.Client { return c.client }
func (c *BranchVTFileContainer) SupportsSubDirs() bool     { return true }
func (c *BranchVTFileContainer) GetInode() *fs.Inode       { return &c.Inode }
func (c *BranchVTFileContainer) StopAutoRefresh()          {}
