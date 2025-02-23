package valfs

import (
	"context"
	"syscall"
	"time"

	common "github.com/404wolf/valfs/common"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// ValFileFlagsExecute defines the file permissions and type for executable val files
const ValFileFlagsExecute = syscall.S_IFREG | 0o777

// ValFileFlagsNoExecute defines the file permissions and type for non-executable val files
const ValFileFlagsNoExecute = syscall.S_IFREG | 0o666

// ValVTFileInode represents a file in the filesystem that corresponds to a val.
// It delegates val operations to a ValVTFile implementation.
type ValVTFileInode struct {
	fs.Inode

	modifiedAt time.Time       // Last modification timestamp
	vtFile     ValVTFile       // Underlying val implementation
	client     *common.Client  // Client for API operations
	parent     VTFileContainer // Parent directory containing this val file
}

// Interface compliance checks
var _ = (fs.NodeSetattrer)((*ValVTFileInode)(nil))
var _ = (fs.NodeGetattrer)((*ValVTFileInode)(nil))
var _ = (fs.NodeWriter)((*ValVTFileInode)(nil))
var _ = (fs.NodeOpener)((*ValVTFileInode)(nil))
var _ = (fs.FileReader)((*ValVTFileHandle)(nil))

// NewValVTFileInode creates a new ValFile that wraps a ValVTFile
// implementation
func NewValVTFileInode(
	vtFile ValVTFile,
	client *common.Client,
	parent VTFileContainer,
) (*ValVTFileInode, error) {
	return &ValVTFileInode{
		vtFile:     vtFile,
		client:     client,
		parent:     parent,
		modifiedAt: time.Now(),
	}, nil
}

// ValVTFileHandle represents an open file handle
type ValVTFileHandle struct {
	valFile *ValVTFileInode
	client  *common.Client
}

// ModifiedNow updates the file's modification time to current time
func (f *ValVTFileInode) ModifiedNow() {
	f.modifiedAt = time.Now()
}

// Open handles opening the file and creates a new file handle
func (f *ValVTFileInode) Open(ctx context.Context, openFlags uint32) (
	fh fs.FileHandle,
	fuseFlags uint32,
	errno syscall.Errno,
) {
	err := f.vtFile.Load(ctx)
	if err != nil {
		common.Logger.Error("Error fetching val", "error", err)
		return nil, 0, syscall.EIO
	}

	common.Logger.Info("Opening val file", "name", f.vtFile.GetName())

	fh = &ValVTFileHandle{
		valFile: f,
		client:  f.client,
	}

	return fh, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

// Read handles reading data from the file
func (fh *ValVTFileHandle) Read(
	ctx context.Context,
	dest []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	err := fh.valFile.vtFile.Load(ctx)
	if err != nil {
		return nil, syscall.EIO
	}

	// Get the full content using GetData which returns the text representation
	packedText, err := fh.valFile.vtFile.GetAsPackedText()
	if err != nil {
		common.Logger.Error("Error fetching val", "error", err)
		return nil, syscall.EIO
	}

	if packedText == nil {
		return fuse.ReadResultData([]byte{}), syscall.F_OK
	}

	end := off + int64(len(dest))
	if end > int64(len(*packedText)) {
		end = int64(len(*packedText))
	}
	return fuse.ReadResultData([]byte((*packedText)[off:end])), syscall.F_OK
}

// Write handles writing data to the file
func (f *ValVTFileInode) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (written uint32, errno syscall.Errno) {
	err := f.vtFile.Load(ctx)
	if err != nil {
		return 0, syscall.EIO
	}

	// Update the val's data
	err = f.vtFile.UpdateFromPackedText(ctx, string(data))
	if err != nil {
		common.Logger.Errorf("Error updating val, parsing error: %s", err)
		return 0, syscall.EIO
	}

	// Save changes
	err = f.vtFile.Save(ctx)
	if err != nil {
		common.Logger.Errorf("Error updating val, error: %s", err)
		return 0, syscall.EIO
	}

	if !f.client.Config.StaticMeta {
		err = f.vtFile.Load(ctx)
		if err != nil {
			return 0, syscall.EIO
		}
		f.ModifiedNow()
	}

	filename := ConstructValFilename(f.vtFile.GetName(), f.vtFile.GetType())
	waitThenMaybeDenoCache(filename, f.client)

	return uint32(len(data)), syscall.F_OK
}

// Getattr retrieves the file attributes
func (f *ValVTFileInode) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	common.Logger.Info("Getting attributes for val file", "name", f.vtFile.GetName())

	// Set size if we have the data
	if f.vtFile.GetAuthorId() != nil {
		packedText, err := f.vtFile.GetAsPackedText()
		if err == nil && packedText != nil {
			out.Size = uint64(len(*packedText))
		}
	}

	f.assignValMode(out)

	modified := &f.modifiedAt
	out.SetTimes(modified, modified, modified)

	return syscall.F_OK
}

// Setattr sets the file attributes
func (f *ValVTFileInode) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	common.Logger.Info("Setting attributes for val file", "name", f.vtFile.GetName())

	out.Size = in.Size
	f.assignValMode(out)
	out.Atime = in.Atime
	out.Mtime = in.Mtime
	out.Ctime = in.Ctime

	return syscall.F_OK
}

func (f *ValVTFileInode) assignValMode(out *fuse.AttrOut) {
	if f.client.Config.ExecutableVals {
		out.Mode = ValFileFlagsExecute
	} else {
		out.Mode = ValFileFlagsNoExecute
	}
}
