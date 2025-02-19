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

const WAIT_BEFORE_DENO_CACHING = time.Second * 1

// The folder with all of my vals in it
type ValsDir struct {
	fs.Inode

	client *common.Client
}

var _ = (fs.NodeRenamer)((*ValsDir)(nil))
var _ = (fs.NodeCreater)((*ValsDir)(nil))
var _ = (fs.NodeUnlinker)((*ValsDir)(nil))

// Set up background refresh of vals and retreive an auto updating folder of
// val files
func NewVals(parent *fs.Inode, client *common.Client, ctx context.Context) *ValsDir {
	valsDir := &ValsDir{client: client}
	attrs := fs.StableAttr{Mode: syscall.S_IFDIR | 0555}
	parent.NewPersistentInode(ctx, valsDir, attrs)

	refreshVals(ctx, &valsDir.Inode, *client)
	if client.Config.AutoRefresh {
		ticker := time.NewTicker(time.Duration(client.Config.AutoRefreshInterval) * time.Second)
		go func() {
			for range ticker.C {
				refreshVals(ctx, &valsDir.Inode, *client)
				common.Logger.Info("Refreshed vals")
			}
		}()
	}

	return valsDir
}

// Handle deletion of a file by also deleting the val
func (c *ValsDir) Unlink(ctx context.Context, name string) syscall.Errno {
	common.Logger.Infof("Deleting val %s", name)
	child := c.GetChild(name)
	if child == nil {
		return syscall.ENOENT
	}

	// Cast the file handle to a ValFile
	valFile, ok := child.Operations().(*ValFile)
	if !ok {
		return syscall.EINVAL
	}

	_, err := c.client.APIClient.ValsAPI.ValsDelete(ctx, valFile.BasicData.Id).Execute()
	if err != nil {
		common.Logger.Error("Error deleting val", err)
		return syscall.EIO
	}
	common.Logger.Infof("Deleted val %s", valFile.BasicData.Id)

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
	valName, valType := ExtractFromFilename(name)
	if valType == Unknown {
		return nil, nil, 0, syscall.EINVAL
	}
	common.Logger.Infof("Creating val %s of type %s", valName, valType)

	// Make a request to create the val
	templateCode := GetTemplate(valType)
	createReq := valgo.NewValsCreateRequest(string(templateCode))
	createReq.SetName(valName)
	createReq.SetType(string(valType))
	createReq.SetPrivacy(DefaultPrivacy)
	val, _, err := c.client.APIClient.ValsAPI.ValsCreate(ctx).ValsCreateRequest(*createReq).Execute()

	// Check if the request was successful
	if err != nil {
		common.Logger.Error("Error creating val", err)
		return nil, nil, 0, syscall.EIO
	} else {
		common.Logger.Infof("Created val %v", val)
	}

	// Create a val file that we can hand over
	valFile, err := NewValFileFromExtendedVal(*val, c.client)
	if err != nil {
		common.Logger.Error("Error creating val file", err)
		return nil, nil, 0, syscall.EIO
	}
	newInode := c.NewPersistentInode(
		ctx,
		valFile,
		fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})

	// Open the file handle
	fileHandle, _, _ := valFile.Open(ctx, flags)
	valFile.ModifiedNow()

	// Schedule a deno cache for after the file gets created to cache new modules
	time.AfterFunc(
		WAIT_BEFORE_DENO_CACHING*time.Millisecond,
		func() { c.client.DenoCacher.DenoCache(name) },
	)

	// Create a file handle
	return newInode, &fileHandle, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

// Rename a val, and change the name in valtown
func (c *ValsDir) Rename(
	ctx context.Context,
	oldName string,
	newParent fs.InodeEmbedder,
	newName string,
	code uint32,
) syscall.Errno {
	// Check if we're moving between directories
	if newParent.EmbeddedInode().StableAttr().Ino != c.Inode.StableAttr().Ino {
		common.Logger.Info("Cannot move val out of the `vals` directory")
		return syscall.EINVAL
	}

	// Validate the new filename
	valName, valType := ExtractFromFilename(newName)
	if valType == Unknown {
		return syscall.EINVAL
	}

	// Check if the new filename already exists
	if c.GetChild(newName) != nil {
		return syscall.EEXIST
	}

	inode := c.GetChild(oldName)
	if inode == nil {
		return syscall.ENOENT
	}
	valFile := inode.Operations().(*ValFile)

	// Prepare the update request
	valUpdateReq := valgo.NewValsUpdateRequest()
	valUpdateReq.SetName(valName)
	valUpdateReq.SetType(string(valType))

	// Update the val in the backend
	_, err := c.client.APIClient.ValsAPI.ValsUpdate(ctx, valFile.BasicData.Id).ValsUpdateRequest(*valUpdateReq).Execute()
	if err != nil {
		common.Logger.Errorf("Error updating val %s: %v", oldName, err)
		return syscall.EIO
	}

	// Fetch what the change produced
	extVal, _, err := c.client.APIClient.ValsAPI.ValsGet(ctx, valFile.BasicData.Id).Execute()
	if err != nil {
		common.Logger.Error("Error fetching val", err)
		return syscall.EIO
	}

	// Update the val file with the new data
	valFile.ExtendedData = extVal
	valFile.BasicData.Name = valName
	valFile.BasicData.Type = string(valType)

	return syscall.F_OK
}
