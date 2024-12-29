package fuse

import (
	"context"
	"log"
	"net/http"
	"syscall"

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
	packed, err := valPackage.ToText()
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
	// Make sure the file handle is valid
	handle, ok := fh.(*BytesFileHandle)
	if !ok {
		return 0, syscall.EBADF
	}

	// Make sure not writing out of bounds
	oldData := handle.content
	if int(off) > len(oldData) {
		return 0, syscall.EINVAL
	}

	// Extract the new data
	newDataLen := int64(len(data))
	nowIsLarger := (off + newDataLen) > int64(len(oldData))

	// If they are growing the file
	if nowIsLarger {
		newDataLen = off + int64(len(data))
	}

	// Create a buffer and copy over data
	newData := make([]byte, newDataLen)
	copy(newData, oldData)        // copy old data to the new data buffer
	copy(newData[off:], data)     // copy new data to the proper location of new data
	newDataStr := string(newData) // convert it to a string

	// Put the new data into the val
	newValPackage := NewValPackage(&c.ValData)
	newValPackage.UpdateVal(newDataStr)

	// Create new packed file contents
	newPackedCode, err := newValPackage.ToText()
	if err != nil {
		return 0, syscall.EIO
	}

	// Update the file handle stored code
	handle.content = []byte(newPackedCode)

	// Update the code of the actual val
	valCreateReqData := valgo.NewValsCreateRequest(c.ValData.GetCode())

	// The things the user can change in the yaml metadata
	valCreateReqData.SetPrivacy(c.ValData.GetPrivacy())
	valCreateReqData.SetReadme(c.ValData.GetReadme())

	// Make the request
	valCreateReq := c.ValClient.ValsAPI.ValsCreateVersion(ctx, c.ValData.GetId()).ValsCreateRequest(*valCreateReqData)
	extVal, resp, err := valCreateReq.Execute()
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Println("Error updating val", err)
		return 0, syscall.EIO
	}

	// Update the val to the new updated val
	c.ValData = *extVal

	// We wrote all new data
	return uint32(newDataLen), syscall.Errno(0)
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
	contentLen, err := valPackage.Len()
	if err != nil {
		return syscall.EIO
	}

	out.Size = uint64(contentLen)
	out.Mode = ValFileMode

	// Set timestamps to be modified now
	modified := f.ValData.VersionCreatedAt
	out.SetTimes(modified, modified, modified)

	return syscall.F_OK
}

var _ = (fs.NodeSetattrer)((*ValFile)(nil))

// Accept the request to change attrs, but ignore the new attrs, to comply with
// editors expecting to be able to change them
func (f *ValFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	return 0
}
