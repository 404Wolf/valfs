package fuse

import (
	"context"
	"log"
	"syscall"
	"time"

	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const ValFileMode = syscall.S_IFREG | 0o777

// A file in the val file system, with metadata about the file and an inode
type ValFile struct {
	fs.Inode
	ValData   valgo.ExtendedVal
	ValClient *valgo.APIClient
}

// A file handle that carries separate content for each open call
type BytesFileHandle struct {
	content []byte
}

var _ = (fs.NodeOpener)((*ValFile)(nil))

// Get a file descriptor for a val file
func (f *ValFile) Open(ctx context.Context, openFlags uint32) (
	fh fs.FileHandle,
	fuseFlags uint32,
	errno syscall.Errno,
) {
	// Provide the Val's code as the data
	valPackage := ValPackage{Val: &f.ValData}
	packed, err := valPackage.GetContents()
	if err != nil {
		return nil, 0, syscall.EIO
	}

	fh = &BytesFileHandle{
		content: []byte(packed),
	}

	// Return FOPEN_DIRECT_IO so content is not cached
	return fh, fuse.FOPEN_DIRECT_IO, 0
}

var _ = (fs.FileReader)((*BytesFileHandle)(nil))

// Provide the content of the val as the content of the file
func (fh *BytesFileHandle) Read(
	ctx context.Context,
	dest []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	end := off + int64(len(dest))
	if end > int64(len(fh.content)) {
		end = int64(len(fh.content))
	}
	log.Printf("Reading from %d to %d", off, end)

	return fuse.ReadResultData(fh.content[off:end]), 0
}

var _ = (fs.NodeWriter)((*ValFile)(nil))

// Write data to a val file and the corresponding val
func (c *ValFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (written uint32, errno syscall.Errno) {
	oldData := (fh.(*BytesFileHandle)).content
	newData := append(oldData[:off], data...)

	valData := &c.ValData
	valData.SetCode(string(newData))

	valCreateReqData := valgo.NewValsCreateRequest(valData.GetCode())
	valCreateReqData.SetName(valData.GetName())
	valCreateReqData.SetType(valData.GetType())

	valCreateReq := c.ValClient.ValsAPI.ValsCreateVersion(ctx, valData.GetId())
	valCreateReq.ValsCreateRequest(*valCreateReqData)

	extVal, resp, err := valCreateReq.Execute()
	if err != nil || resp.StatusCode != 200 {
		return 0, syscall.EIO
	}
	c.ValData = *extVal

	return uint32(len(data)), 0
}

var _ = (fs.NodeGetattrer)((*ValFile)(nil))

// Make sure the file is always read/write/executable even if changed
func (f *ValFile) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	// Set the mode to indicate a regular file with read, write, and execute
	// permissions for all
	valPackage := ValPackage{Val: &f.ValData}
	contentLen, err := valPackage.GetContentsLength()
	if err != nil {
		return syscall.EIO
	}

	out.Mode = syscall.S_IFDIR | ValFileMode
	out.Size = uint64(contentLen)

	// Set timestamps to be modified now
	now := time.Now()
	out.SetTimes(&now, &now, &now)

	return syscall.F_OK
}
