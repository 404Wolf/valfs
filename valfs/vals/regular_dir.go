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

// RegularValsDir is the folder where all regular val town vals are located
type RegularValsDir struct {
	fs.Inode

	client *common.Client
	config common.RefresherConfig
}

var _ = (fs.NodeRenamer)((*RegularValsDir)(nil))
var _ = (fs.NodeCreater)((*RegularValsDir)(nil))
var _ = (fs.NodeUnlinker)((*RegularValsDir)(nil))
var _ = (VTFileContainer)((*RegularValsDir)(nil))

var previousIds = make(map[string]*ValVTFileInode)

// Set up background refresh of vals and retreive an auto updating folder of
// val files
func NewRegularValsDir(
	parent *fs.Inode,
	client *common.Client,
	ctx context.Context,
) VTFileContainer {
	common.Logger.Info("Initializing new ValsDir")
	valsDir := &RegularValsDir{
		client: client,
		config: common.RefresherConfig{LookupCap: 99},
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
func (c *RegularValsDir) GetClient() *common.Client {
	return c.client
}

// SupportsSubDirs returns whether the vals dir supports subdirectories
func (c *RegularValsDir) SupportsSubDirs() bool {
	return false
}

// GetInode returns the inode associated with this ValsContainer
func (c *RegularValsDir) GetInode() *fs.Inode {
	return &c.Inode
}

// Unlink handles deletion of a file by also deleting the val
func (c *RegularValsDir) Unlink(ctx context.Context, name string) syscall.Errno {
	common.Logger.Infof("Unlink request received for val: %s", name)
	child := c.GetChild(name)
	if child == nil {
		common.Logger.Warnf("Unlink failed: val %s not found", name)
		return syscall.ENOENT
	}

	valFile, ok := child.Operations().(*ValVTFileInode)
	if !ok {
		common.Logger.Errorf("Unlink failed: %s is not a ValVTFileInode", name)
		return syscall.EINVAL
	}

	common.Logger.Infof("Attempting to delete val %s (ID: %s)", name, valFile.vtFile.GetId())
	err := DeleteValVTFile(ctx, c.client.APIClient, valFile.vtFile.GetId())
	if err != nil {
		common.Logger.Errorf("Error deleting val %s: %v", name, err)
		return syscall.EIO
	}

	delete(previousIds, name)
	return syscall.F_OK
}

// Create a new val on new file creation
func (c *RegularValsDir) Create(
	ctx context.Context,
	name string,
	flags uint32,
	mode uint32,
	entryOut *fuse.EntryOut,
) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, code syscall.Errno) {
	common.Logger.Infof("Create request received for: %s", name)

	valName, valType := ExtractFromValFilename(name)
	if valType == VTFileTypeUnknown {
		common.Logger.Errorf("Create failed: unknown val type for file %s", name)
		return nil, nil, 0, syscall.EINVAL
	}

	templateCode := GetTemplate(valType)
	common.Logger.Infof("Creating val %s of type %s", valName, valType)

	vtFile, err := CreateNewValVTFile(
		ctx,
		c.client.APIClient,
		valType,
		string(templateCode),
		valName,
		DefaultValPrivacy,
	)
	if err != nil {
		common.Logger.Errorf("API error creating val %s: %v", name, err)
		return nil, nil, 0, syscall.EIO
	}

	valFile, err := NewValVTFileInode(vtFile, c.client, c)
	if err != nil {
		common.Logger.Errorf("Error creating val file for %s: %v", name, err)
		return nil, nil, 0, syscall.EIO
	}

	newInode := c.NewPersistentInode(
		ctx,
		valFile,
		fs.StableAttr{Mode: syscall.S_IFREG})

	fileHandle, _, _ := valFile.Open(ctx, flags)
	valFile.ModifiedNow()

	waitThenMaybeDenoCache(name, c.client)

	return newInode, fileHandle, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

// Rename a val, and change the name in valtown
func (c *RegularValsDir) Rename(
	ctx context.Context,
	oldName string,
	newParent fs.InodeEmbedder,
	newName string,
	flags uint32,
) syscall.Errno {
	common.Logger.Infof("Rename request from %s to %s", oldName, newName)

	if newParent.EmbeddedInode().StableAttr().Ino != c.Inode.StableAttr().Ino {
		common.Logger.Warn("Cannot move val out of the vals directory")
		return syscall.EINVAL
	}

	valName, valType := ExtractFromValFilename(newName)
	if valType == VTFileTypeUnknown {
		common.Logger.Errorf("Invalid val type in new name: %s", newName)
		return syscall.EINVAL
	}

	inode := c.GetChild(oldName)
	if inode == nil {
		common.Logger.Warnf("Source file not found: %s", oldName)
		return syscall.ENOENT
	}

	valFile := inode.Operations().(*ValVTFileInode)
	valFile.vtFile.SetName(valName)
	valFile.vtFile.SetType(string(valType))
	err := valFile.vtFile.Save(ctx)
	if err != nil {
		common.Logger.Errorf("Error updating val %s: %v", oldName, err)
		return syscall.EIO
	}

	return syscall.F_OK
}

// Refresh implements the refresh operation for the vals container
func (c *RegularValsDir) Refresh(ctx context.Context) error {
	common.Logger.Info("Starting refresh operation")
	newVals, err := ListValVTFiles(ctx, c.client.APIClient)
	if err != nil {
		common.Logger.Error("Error fetching vals", err)
		return err
	}

	newValsMap := make(map[string]ValVTFile)
	for _, newVal := range newVals {
		filename := ConstructValFilename(newVal.GetName(), newVal.GetType())
		newValsMap[filename] = newVal
	}
	common.Logger.Infof("Fetched %d vals for refresh", len(newVals))

	for _, newVal := range newVals {
		filename := ConstructValFilename(newVal.GetName(), newVal.GetType())
		prevVal, exists := previousIds[filename]

		if !exists {
			common.Logger.Infof("Creating new val file for %s", filename)
			valFile, err := NewValVTFileInode(newVal, c.client, c)
			if err != nil {
				common.Logger.Errorf("Error creating val file for %s: %v", filename, err)
				continue
			}
			inode := c.NewPersistentInode(ctx, valFile, fs.StableAttr{Mode: syscall.S_IFREG})
			c.AddChild(filename, inode, true)
			previousIds[filename] = valFile
			common.Logger.Infof("Added val %s, found fresh on valtown", filename)
		}

		if exists && newVal.GetVersion() > prevVal.vtFile.GetVersion() {
			common.Logger.Infof("Updating existing val %s to version %d", filename, newVal.GetVersion())
			prevVal.vtFile = newVal
			prevVal.ModifiedNow()
			prevVal.EmbeddedInode().NotifyContent(0, 0)
			common.Logger.Infof("Updated val %s, found newer on valtown", filename)
		}
	}

	// Remove vals that no longer exist
	for filename := range previousIds {
		if _, exists := newValsMap[filename]; !exists {
			common.Logger.Infof("Removing val %s as it's no longer found on valtown", filename)
			c.RmChild(filename)
			delete(previousIds, filename)
			common.Logger.Infof("Removed val %s no longer found on valtown", filename)
		}
	}

	return nil
}

// StartAutoRefresh begins automatic refreshing of the vals container
func (c *RegularValsDir) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	StartAutoRefreshHelper(ctx, interval, c.Refresh)
}
