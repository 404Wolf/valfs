package valfs

import (
	"context"
	"errors"
	"syscall"
	"time"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// VAL_FILE_FLAGS defines the file permissions and type for val files
// S_IFREG indicates a regular file, 0o777 gives full rwx permissions
const VAL_FILE_FLAGS = syscall.S_IFREG | 0o777

// ValFile represents a file in the filesystem that corresponds to a val
type ValFile struct {
	fs.Inode

	ModifiedAt   time.Time          // Last modification timestamp
	BasicData    valgo.BasicVal     // Basic val metadata
	ExtendedData *valgo.ExtendedVal // Full val data (loaded on demand)
	client       *common.Client     // Client for API operations
	parent       ValsContainer      // Parent directory containing this val file
}

// Interface compliance checks
var _ = (fs.NodeSetattrer)((*ValFile)(nil))
var _ = (fs.NodeGetattrer)((*ValFile)(nil))
var _ = (fs.NodeWriter)((*ValFile)(nil))
var _ = (fs.NodeOpener)((*ValFile)(nil))
var _ = (fs.FileReader)((*ValFileHandle)(nil))

// GetExtendedData fetches the full val data if not already loaded
func (f *ValFile) GetExtendedData(ctx context.Context) (*valgo.ExtendedVal, error) {
	if f.ExtendedData == nil {
		extVal, err := f.parent.GetValOps().Read(ctx, f.BasicData.GetId())
		if err != nil {
			return nil, errors.New("Failed to fetch extended data")
		}
		f.ExtendedData = extVal
	}
	return f.ExtendedData, nil
}

// getValVersionCreatedAt retrieves the creation timestamp of a val version
func getValVersionCreatedAt(val valgo.ExtendedVal, client *common.Client) *time.Time {
	modified := val.VersionCreatedAt
	if modified == nil {
		ctx := context.Background()
		versionList, _, err := client.APIClient.ValsAPI.ValsList(ctx,
			val.Id).Offset(0).Limit(1).Execute()
		if err != nil {
			common.Logger.Error("Error fetching version list", err)
		}
		modified = &versionList.Data[0].CreatedAt
	}
	return modified
}

// NewValFileFromBasicVal creates a new ValFile from basic val metadata
func NewValFileFromBasicVal(
	ctx context.Context,
	val valgo.BasicVal,
	client *common.Client,
	parent ValsContainer,
) (*ValFile, error) {
	common.Logger.Info("Create new val file from basic val", "name", val.Name)

	valFile := &ValFile{
		BasicData:  val,
		client:     client,
		parent:     parent,
		ModifiedAt: time.Now(),
	}

	return valFile, nil
}

// NewValFileFromExtendedVal creates a new ValFile from complete val data
func NewValFileFromExtendedVal(
	val valgo.ExtendedVal,
	client *common.Client,
	parent ValsContainer,
) (*ValFile, error) {
	common.Logger.Info("Create new val file from extended val", "name", val.Name)

	return &ValFile{
		BasicData:    val.ToBasicVal(),
		ExtendedData: &val,
		client:       client,
		parent:       parent,
		ModifiedAt:   *getValVersionCreatedAt(val, client),
	}, nil
}

// ValFileHandle represents an open file handle
type ValFileHandle struct {
	ValFile *ValFile
	client  *common.Client
}

// ModifiedNow updates the file's modification time to current time
func (f *ValFile) ModifiedNow() {
	f.ModifiedAt = time.Now()
}

// Open handles opening the file and creates a new file handle
func (f *ValFile) Open(ctx context.Context, openFlags uint32) (
	fh fs.FileHandle,
	fuseFlags uint32,
	errno syscall.Errno,
) {
	// Load extended data if not already loaded
	if f.ExtendedData == nil {
		common.Logger.Info("Valfile was lazy. Now getting extended val data", "name", f.BasicData.Name)
		extVal, err := f.parent.GetValOps().Read(ctx, f.BasicData.Id)
		if err != nil {
			common.Logger.Error("Error fetching val", "error", err)
			return nil, 0, syscall.EIO
		}
		f.ExtendedData = extVal
	}
	common.Logger.Info("Opening val file", "name", f.BasicData.Name)

	fh = &ValFileHandle{
		ValFile: f,
		client:  f.client,
	}

	filename := ConstructFilename(f.BasicData.GetName(), ValType(f.BasicData.GetType()))
	waitThenMaybeDenoCache(filename, f.client)

	return fh, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

// Read handles reading data from the file
func (fh *ValFileHandle) Read(
	ctx context.Context,
	dest []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	extVal, err := fh.ValFile.GetExtendedData(ctx)
	if err != nil {
		return nil, syscall.EIO
	}
	common.Logger.Info("Reading val file", "val", extVal)

	valPackage := NewValPackage(fh.client, extVal)
	content, err := valPackage.ToText()
	if err != nil {
		return nil, syscall.EIO
	}
	bytes := []byte(*content)

	end := off + int64(len(dest))
	if end > int64(len(bytes)) {
		end = int64(len(bytes))
	}
	return fuse.ReadResultData(bytes[off:end]), syscall.F_OK
}

// Write handles writing data to the file
func (c *ValFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (written uint32, errno syscall.Errno) {
	prevExtVal, err := c.GetExtendedData(ctx)
	common.Logger.Info("Writing to val file", "val", prevExtVal.GetId())

	newValPackage := NewValPackage(c.client, prevExtVal)
	if err != nil {
		common.Logger.Error("Error updating val package", "error", err)
		return 0, syscall.EIO
	}
	newValPackage.UpdateVal(string(data))
	extVal := newValPackage.Val

	err = c.parent.GetValOps().UpdateCode(ctx, prevExtVal.GetId(), newValPackage.Val.GetCode())
	if err != nil {
		common.Logger.Error("Error updating val code", "error", err)
		return 0, syscall.EIO
	}

	// Update metadata based on configuration
	if !c.client.Config.StaticMeta {
		extVal, err = c.parent.GetValOps().Read(ctx, prevExtVal.GetId())
		if err != nil {
			common.Logger.Error("Error fetching val", "error", err)
			return 0, syscall.EIO
		}
		c.ExtendedData = extVal
		c.ModifiedNow()
	} else {
		c.ModifiedAt = time.Now().Add(-1 * time.Second)
	}

	filename := ConstructFilename(c.BasicData.GetName(), ValType(c.BasicData.GetType()))
	waitThenMaybeDenoCache(filename, c.client)

	return uint32(len(data)), syscall.F_OK
}

// Getattr retrieves the file attributes
func (f *ValFile) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	common.Logger.Info("Getting attributes for val file", "name", f.BasicData.Name)

	// Handle basic attributes if extended data isn't loaded
	if f.ExtendedData == nil {
		out.Mode = VAL_FILE_FLAGS
		out.Size = uint64(len(f.BasicData.GetCode()) + 500)
		modified := time.Unix(0, 0)
		out.SetTimes(&modified, &modified, &modified)
		return 0
	}

	valPackage := NewValPackage(f.client, f.ExtendedData)
	contentLen, err := valPackage.Len()
	if err != nil {
		common.Logger.Error("Error getting content length", "error", err)
		return syscall.EIO
	}

	out.Size = uint64(contentLen)
	out.Mode = VAL_FILE_FLAGS

	modified := &f.ModifiedAt
	out.SetTimes(modified, modified, modified)

	common.Logger.Info("Got attributes for val file",
		"name", f.BasicData.Name,
		"size", out.Size,
		"mode", out.Mode,
		"modified", *modified)

	return syscall.F_OK
}

// Setattr sets the file attributes
func (f *ValFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	common.Logger.Info("Setting attributes for val file", "name", f.BasicData.Name)

	out.Size = in.Size
	out.Mode = VAL_FILE_FLAGS
	out.Atime = in.Atime
	out.Mtime = in.Mtime
	out.Ctime = in.Ctime

	return syscall.F_OK
}
