package fuse

import (
	"context"
	"log"
	"net/http"
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
	ModifiedAt time.Time
	ValData    valgo.ExtendedVal
	ValClient  *valgo.APIClient
}

// Create a new val file, but does not attach an inode embedding
func NewValFile(
	val valgo.ExtendedVal,
	client *valgo.APIClient,
) (*ValFile, error) {
	// Get the last modified date to cache
	modified := val.VersionCreatedAt
	if modified == nil {
		ctx := context.Background()
		versionList, resp, err := client.ValsAPI.ValsList(ctx, val.Id).Offset(0).Limit(1).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Println("Error fetching version list", err)
			return nil, err
		}
		modified = &versionList.Data[0].CreatedAt
	}

	return &ValFile{
		ValData:    val,
		ValClient:  client,
		ModifiedAt: *modified,
	}, nil
}

// A file handle that carries separate content for each open call
type ValFileHandle struct {
	ValFile *ValFile
}

// Update modified time to be now
func (f *ValFile) ModifiedNow() {
	f.ModifiedAt = time.Now()
}

var _ = (fs.NodeOpener)((*ValFile)(nil))

// Get a file descriptor for a val file
func (f *ValFile) Open(ctx context.Context, openFlags uint32) (
	fh fs.FileHandle,
	fuseFlags uint32,
	errno syscall.Errno,
) {
	fh = &ValFileHandle{
		ValFile: f,
	}

	// Return FOPEN_DIRECT_IO so content is not cached
	return fh, fuse.FOPEN_DIRECT_IO, 0
}

var _ = (fs.FileReader)((*ValFileHandle)(nil))

// Provide the content of the val as the content of the file
func (fh *ValFileHandle) Read(
	ctx context.Context,
	dest []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	// Provide the Val's code as the data
	valPackage := ValPackage{Val: &fh.ValFile.ValData}
	content, err := valPackage.ToText()
	if err != nil {
		return nil, syscall.EIO
	}
	bytes := []byte(*content)

	end := off + int64(len(dest))
	if end > int64(len(bytes)) {
		end = int64(len(bytes))
	}
	log.Printf("Reading from %d to %d", off, end)

	return fuse.ReadResultData(bytes[off:end]), 0
}

var _ = (fs.NodeWriter)((*ValFile)(nil))

// Write data to a val file and the corresponding val
func (c *ValFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (written uint32, errno syscall.Errno) {
	go func() {
		// Create new packed file contents
		newValPackage := ValPackage{Val: &c.ValData}
		err := newValPackage.UpdateVal(string(data))
		if err != nil {
			log.Println("Error updating val package", err)
			return
		}

		// The things the user can change in the yaml metadata
		valCreateReqData := valgo.NewValsCreateRequest(newValPackage.Val.GetCode())
		valCreateReqData.SetPrivacy(c.ValData.GetPrivacy())
		valCreateReqData.SetReadme(c.ValData.GetReadme())

		// Make the request to update the val
		valCreateReq := c.ValClient.ValsAPI.ValsCreateVersion(ctx, c.ValData.GetId()).ValsCreateRequest(*valCreateReqData)
		extVal, resp, err := valCreateReq.Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Println("Error updating val", err)
		} else {
			log.Println("Successfully updated val")
		}

		// Update the val to the new updated val
		c.ValData = *extVal
		c.NotifyContent(0, int64(len(data)))
		c.ModifiedNow()
	}()

	// We are writing all the new data but to prevent lag we want to say we wrote
	// right away
	c.NotifyContent(0, int64(len(data)))
	return uint32(len(data)), syscall.Errno(0)
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
		log.Println("Error getting content length", err)
		return syscall.EIO
	}

	out.Size = uint64(contentLen)
	out.Mode = ValFileMode

	// Set timestamps to be modified now
	modified := &f.ModifiedAt
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
	out.Size = in.Size
	out.Mode = ValFileMode
	out.Atime = in.Atime
	out.Mtime = in.Mtime
	out.Ctime = in.Ctime

	return syscall.F_OK
}
