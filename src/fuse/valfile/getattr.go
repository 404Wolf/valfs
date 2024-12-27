package fuse

import (
	"context"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

var _ = (fs.NodeGetattrer)((*ValFile)(nil))

// Make sure the file is always read/write/executable even if changed
func (f *ValFile) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	// Set the mode to indicate a regular file with read, write, and execute
	// permissions for all
	out.Mode = syscall.S_IFDIR | ValFileMode
	out.Size = uint64(len(getContents(&f.ValData)))

	// Set timestamps
	now := time.Now()
	out.SetTimes(&now, &now, &now)

	return syscall.F_OK
}
