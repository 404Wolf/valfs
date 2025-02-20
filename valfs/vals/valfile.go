package valfs

import (
	"context"
	"syscall"
	"time"

	common "github.com/404wolf/valfs/common"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// ValFileFlags defines the file permissions and type for val files
// S_IFREG indicates a regular file, 0o777 gives full rwx permissions
const ValFileFlagsExecute = syscall.S_IFREG | 0o777
const ValFileFlagsNoExecute = syscall.S_IFREG | 0o666

// ValFile represents a file in the filesystem that corresponds to a val
type ValFile struct {
	fs.Inode

	ModifiedAt time.Time      // Last modification timestamp
	Val        Val            // Val data and operations
	client     *common.Client // Client for API operations
	parent     ValsContainer  // Parent directory containing this val file
}

// Interface compliance checks
var _ = (fs.NodeSetattrer)((*ValFile)(nil))
var _ = (fs.NodeGetattrer)((*ValFile)(nil))
var _ = (fs.NodeWriter)((*ValFile)(nil))
var _ = (fs.NodeOpener)((*ValFile)(nil))
var _ = (fs.FileReader)((*ValFileHandle)(nil))

// NewValFileFromVal creates a new ValFile from complete val data
func NewValFile(
	val Val,
	client *common.Client,
	parent ValsContainer,
) (*ValFile, error) {
	return &ValFile{
		Val:        val,
		client:     client,
		parent:     parent,
		ModifiedAt: time.Now(),
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
	err := f.Val.Get(ctx)
	if err != nil {
		common.Logger.Error("Error fetching val", "error", err)
		return nil, 0, syscall.EIO
	}

	common.Logger.Info("Opening val file", "name", f.Val.GetName())

	fh = &ValFileHandle{
		ValFile: f,
		client:  f.client,
	}

	return fh, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

// Read handles reading data from the file
func (fh *ValFileHandle) Read(
	ctx context.Context,
	dest []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	err := fh.ValFile.Val.Get(ctx)
	if err != nil {
		return nil, syscall.EIO
	}

	valPackage := NewValPackage(fh.client, fh.ValFile.Val)
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
	err := c.Val.Get(ctx)
	if err != nil {
		return 0, syscall.EIO
	}

	newValPackage := NewValPackage(c.client, c.Val)
	err = newValPackage.UpdateVal(string(data))

	if err != nil && off != 0 {
		common.Logger.Error("Bad input ", err)
		return 0, syscall.EINVAL
	}

	err = c.Val.Update(ctx)
	if err != nil {
		common.Logger.Error("Error updating val", "error", err)
		return 0, syscall.EIO
	}

	if !c.client.Config.StaticMeta {
		err = c.Val.Get(ctx)
		if err != nil {
			return 0, syscall.EIO
		}
		c.ModifiedNow()
	} else {
		c.ModifiedAt = time.Now().Add(-1 * time.Second)
	}

	filename := ConstructFilename(c.Val.GetName(), c.Val.GetValType())
	waitThenMaybeDenoCache(filename, c.client)

	return uint32(len(data)), syscall.F_OK
}

// Getattr retrieves the file attributes
func (f *ValFile) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	common.Logger.Info("Getting attributes for val file", "name", f.Val.GetName())

	valPackage := NewValPackage(f.client, f.Val)
	contentLen, err := valPackage.Len()
	if err != nil {
		common.Logger.Error("Error getting content length", "error", err)
		return syscall.EIO
	}

	out.Size = uint64(contentLen)
	f.assignValMode(out)

	modified := &f.ModifiedAt
	out.SetTimes(modified, modified, modified)

	return syscall.F_OK
}

// Setattr sets the file attributes
func (f *ValFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	common.Logger.Info("Setting attributes for val file", "name", f.Val.GetName())

	out.Size = in.Size
	f.assignValMode(out)
	out.Atime = in.Atime
	out.Mtime = in.Mtime
	out.Ctime = in.Ctime

	return syscall.F_OK
}

func (f *ValFile) assignValMode(out *fuse.AttrOut) {
	if f.client.Config.ExecutableVals {
		out.Mode = ValFileFlagsExecute
	} else {
		out.Mode = ValFileFlagsNoExecute
	}
}
