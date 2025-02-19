package valfs

import (
	"context"
	"syscall"
	"time"

	_ "embed"

	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	common "github.com/404wolf/valfs/common"
)

// The folder with all of my vals in it
type ValsDir struct {
	fs.Inode

	client   *common.Client
	config   common.RefresherConfig
	stopChan chan struct{}
	valOps   ValOperations
}

var _ = (fs.NodeRenamer)((*ValsDir)(nil))
var _ = (fs.NodeCreater)((*ValsDir)(nil))
var _ = (fs.NodeUnlinker)((*ValsDir)(nil))
var _ = (ValsContainer)((*ValsDir)(nil))

var previousValIds = make(map[string]*ValFile)

// Set up background refresh of vals and retreive an auto updating folder of
// val files
func NewValsDir(
	parent *fs.Inode,
	client *common.Client,
	ctx context.Context,
) ValsContainer {
	common.Logger.Info("Initializing new ValsDir")
	valsDir := &ValsDir{
		client:   client,
		valOps:   NewValDirOperations(client),
		config:   common.RefresherConfig{LookupCap: 99},
		stopChan: nil,
	}

	// Add the inode to the parent
	attrs := fs.StableAttr{Mode: syscall.S_IFDIR | 0555}
	parent.NewPersistentInode(ctx, valsDir, attrs)

	// Initial refresh
	common.Logger.Info("Performing initial refresh of ValsDir")
	valsDir.Refresh(ctx)

	// Start auto-refresh if configured
	if client.Config.AutoRefresh {
		interval := time.Duration(client.Config.AutoRefreshInterval) * time.Second
		common.Logger.Infof("Starting auto-refresh with interval: %v", interval)
		valsDir.StartAutoRefresh(ctx, interval)
	} else {
		common.Logger.Info("Auto-refresh is disabled by configuration")
	}

	return valsDir
}

// GetClient returns the client associated with the ValsDir
func (c *ValsDir) GetClient() *common.Client {
	return c.client
}

// GetValOps returns the val operations (CRUD API abstractions) associated with
// the ValsDir
func (c *ValsDir) GetValOps() ValOperations {
	return c.valOps
}

// GetClient returns whether the vals dir supports subdirectories
func (c *ValsDir) SupportsDirs() bool {
	return false
}

// GetInode returns the inode associated with this ValsContainer
func (c *ValsDir) GetInode() *fs.Inode {
	return &c.Inode
}

// Handle deletion of a file by also deleting the val
func (c *ValsDir) Unlink(ctx context.Context, name string) syscall.Errno {
	common.Logger.Infof("Unlink request received for val: %s", name)
	child := c.GetChild(name)
	if child == nil {
		common.Logger.Warnf("Unlink failed: val %s not found", name)
		return syscall.ENOENT
	}

	valFile, ok := child.Operations().(*ValFile)
	if !ok {
		common.Logger.Errorf("Unlink failed: %s is not a ValFile", name)
		return syscall.EINVAL
	}

	common.Logger.Infof("Attempting to delete val %s (ID: %s)", name, valFile.BasicData.Id)
	err := c.GetValOps().Delete(ctx, valFile.BasicData.Id)
	if err != nil {
		common.Logger.Errorf("Error deleting val %s: %v", name, err)
		return syscall.EIO
	}
	common.Logger.Infof("Successfully deleted val %s (ID: %s)", valFile.BasicData.Id)

	return 0
}

// Create a new val on new file creation
func (c *ValsDir) Create(
	ctx context.Context,
	name string,
	flags uint32,
	mode uint32,
	entryOut *fuse.EntryOut,
) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, code syscall.Errno) {
	common.Logger.Infof("Create request received for: %s (flags: %d, mode: %d)", name, flags, mode)

	valName, valType := ExtractFromFilename(name)
	if valType == Unknown {
		common.Logger.Errorf("Create failed: unknown val type for file %s", name)
		return nil, nil, 0, syscall.EINVAL
	}

	templateCode := GetTemplate(valType)
	common.Logger.Infof("Creating val %s of type %s with privacy %s", valName, valType, DefaultPrivacy)

	val, err := c.GetValOps().Create(ctx, valName, string(valType), string(templateCode), DefaultPrivacy)
	if err != nil {
		common.Logger.Errorf("API error creating val %s: %v", name, err)
		return nil, nil, 0, syscall.EIO
	}

	common.Logger.Info("Creating val file from extended val")
	valFile, err := NewValFileFromExtendedVal(*val, c.client, c)
	if err != nil {
		common.Logger.Errorf("Error creating val file for %s: %v", name, err)
		return nil, nil, 0, syscall.EIO
	}

	newInode := c.NewPersistentInode(
		ctx,
		valFile,
		fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})

	fileHandle, _, _ := valFile.Open(ctx, flags)
	valFile.ModifiedNow()

	waitThenMaybeDenoCache(name, c.client)

	return newInode, &fileHandle, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

// Rename a val, and change the name in valtown
func (c *ValsDir) Rename(
	ctx context.Context,
	oldName string,
	newParent fs.InodeEmbedder,
	newName string,
	flags uint32,
) syscall.Errno {
	common.Logger.Infof("Rename request from %s to %s", oldName, newName)

	if newParent.EmbeddedInode().StableAttr().Ino != c.Inode.StableAttr().Ino {
		common.Logger.Warn("Cannot move val out of the `vals` directory")
		return syscall.EINVAL
	}

	valName, valType := ExtractFromFilename(newName)
	if valType == Unknown {
		common.Logger.Errorf("Invalid val type in new name: %s", newName)
		return syscall.EINVAL
	}

	if c.GetChild(newName) != nil {
		common.Logger.Warnf("Destination file already exists: %s", newName)
		return syscall.EEXIST
	}

	inode := c.GetChild(oldName)
	if inode == nil {
		common.Logger.Warnf("Source file not found: %s", oldName)
		return syscall.ENOENT
	}
	valFile := inode.Operations().(*ValFile)

	common.Logger.Infof("Updating val %s to new name %s and type %s", oldName, valName, valType)
	extVal, err := c.GetValOps().Update(ctx, valFile.BasicData.Id, valName, string(valType))
	if err != nil {
		common.Logger.Errorf("Error updating val %s: %v", oldName, err)
		return syscall.EIO
	}

	valFile.ExtendedData = extVal
	valFile.BasicData.Name = valName
	valFile.BasicData.Type = string(valType)
	common.Logger.Infof("Successfully renamed val from %s to %s", oldName, newName)

	return syscall.F_OK
}

// Refresh implements the refresh operation for the vals container
func (c *ValsDir) Refresh(ctx context.Context) error {
	common.Logger.Info("Starting refresh operation")
	newVals, err := c.getVals(ctx)
	if err != nil {
		common.Logger.Error("Error fetching vals", err)
		return err
	}

	newValsIdsToBasicVals := make(map[string]valgo.BasicVal)
	for _, newVal := range newVals {
		newValsIdsToBasicVals[newVal.GetId()] = newVal
	}
	common.Logger.Infof("Fetched %d vals for refresh", len(newVals))

	for _, newVal := range newVals {
		prevValFile, exists := previousValIds[newVal.GetId()]

		if !exists {
			common.Logger.Infof("Creating new val file for %s", newVal.GetId())
			valFile, err := NewValFileFromBasicVal(ctx, newVal, c.client, c)
			if err != nil {
				common.Logger.Errorf("Error creating val file for %s: %v", newVal.GetId(), err)
				return err
			}
			filename := ConstructFilename(newVal.GetName(), ValType(newVal.GetType()))
			c.NewPersistentInode(ctx, valFile, fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})
			c.AddChild(filename, &valFile.Inode, true)
			previousValIds[newVal.GetId()] = valFile
			common.Logger.Infof("Added val %s, found fresh on valtown", newVal.GetId())
		}

		if exists && newVal.GetVersion() > prevValFile.BasicData.GetVersion() {
			common.Logger.Infof("Updating existing val %s to version %d", newVal.GetId(), newVal.GetVersion())
			prevValFile.BasicData = newVal
			prevValFile.ModifiedNow()
			prevValFile.EmbeddedInode().Root().NotifyContent(0, 0)
			common.Logger.Infof("Updated val %s, found newer on valtown", newVal.GetId())
		}
	}

	for _, oldVal := range previousValIds {
		if _, exists := newValsIdsToBasicVals[oldVal.BasicData.GetId()]; !exists {
			filename := ConstructFilename(oldVal.BasicData.GetName(), ValType(oldVal.BasicData.GetType()))
			common.Logger.Infof("Removing val %s as it's no longer found on valtown", filename)
			c.RmChild(filename)
			delete(previousValIds, oldVal.BasicData.GetId())
			common.Logger.Infof("Removed val %s no longer found on valtown", oldVal.BasicData.GetId())
		}
	}

	return nil
}

// getVals retrieves a list of all the vals belonging to the authed user
func (c *ValsDir) getVals(ctx context.Context) ([]valgo.BasicVal, error) {
	common.Logger.Info("Fetching all of my vals")

	offset := int32(0)
	allVals := []valgo.BasicVal{}

	for {
		common.Logger.Infof("Fetching vals batch with offset %d", offset)
		vals, err := c.GetValOps().List(ctx, offset, c.config.LookupCap)
		if err != nil {
			common.Logger.Errorf("Error fetching vals batch: %v", err)
			return nil, err
		}

		allVals = append(allVals, vals...)

		if len(vals) < int(c.config.LookupCap) {
			break
		}

		offset += c.config.LookupCap
	}

	common.Logger.Infof("Fetched all of my vals. Found %d vals", len(allVals))
	return allVals, nil
}

// StartAutoRefresh begins automatic refreshing of the vals container
func (c *ValsDir) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	common.Logger.Infof("Starting auto-refresh with interval %v", interval)
	if c.stopChan != nil {
		common.Logger.Info("Stopping existing auto-refresh before starting new one")
		c.StopAutoRefresh()
	}

	c.stopChan = make(chan struct{})
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				common.Logger.Info("Auto-refresh ticker triggered")
				if err := c.Refresh(ctx); err != nil {
					common.Logger.Error("Error refreshing vals:", err)
				} else {
					common.Logger.Info("Successfully completed auto-refresh")
				}
			case <-c.stopChan:
				common.Logger.Info("Auto-refresh stopped")
				ticker.Stop()
				return
			}
		}
	}()
}

// StopAutoRefresh stops the automatic refreshing of the vals container
func (c *ValsDir) StopAutoRefresh() {
	common.Logger.Info("Stopping auto-refresh")
	if c.stopChan != nil {
		close(c.stopChan)
		c.stopChan = nil
		common.Logger.Info("Auto-refresh stopped successfully")
	} else {
		common.Logger.Info("Auto-refresh was not running")
	}
}
