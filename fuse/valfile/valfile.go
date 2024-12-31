package fuse

import (
	"context"
	"log"
	"net/http"
	"syscall"
	"time"

	client "github.com/404wolf/valfs/client"
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
	client     *client.Client
}

// Create a new val file, but does not attach an inode embedding
func NewValFile(
	val valgo.ExtendedVal,
	client *client.Client,
) (*ValFile, error) {
	log.Println("Create new val file named", val.Name)

	// Get the last modified date to cache
	modified := val.VersionCreatedAt
	if modified == nil {
		ctx := context.Background()
		versionList, resp, err := client.APIClient.ValsAPI.ValsList(ctx, val.Id).Offset(0).Limit(1).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Println("Error fetching version list", err)
			return nil, err
		}
		modified = &versionList.Data[0].CreatedAt
	}
	log.Println("Setting new val file modified at to", *modified)

	return &ValFile{
		ValData:    val,
		client:     client,
		ModifiedAt: *modified,
	}, nil
}

// A file handle that carries separate content for each open call
type ValFileHandle struct {
	ValFile *ValFile
	client  *client.Client
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
	log.Println("Opening val file", f.ValData.Name)

	// Create a new file handle for the val
	fh = &ValFileHandle{
		ValFile: f,
		client:  f.client,
	}

	// Return FOPEN_DIRECT_IO so content is not cached
	return fh, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

var _ = (fs.FileReader)((*ValFileHandle)(nil))

// Provide the content of the val as the content of the file
func (fh *ValFileHandle) Read(
	ctx context.Context,
	dest []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	log.Println("Reading val file", fh.ValFile.ValData.Name)

	// Provide the Val's code as the data
	valPackage := NewValPackage(fh.client, &fh.ValFile.ValData)
	content, err := valPackage.ToText()
	if err != nil {
		return nil, syscall.EIO
	}
	bytes := []byte(*content)

	// Get the requested region and return it
	end := off + int64(len(dest))
	if end > int64(len(bytes)) {
		end = int64(len(bytes))
	}
	return fuse.ReadResultData(bytes[off:end]), syscall.F_OK
}

var _ = (fs.NodeWriter)((*ValFile)(nil))

// Write data to a val file and the corresponding val
func (c *ValFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (written uint32, errno syscall.Errno) {
	log.Println("Writing to val file", c.ValData.Name)

	go func() {
		// Commit the writes to val town API
		log.Println("Commiting the write to", c.ValData.Name)

		// Create new packed file contents
		newValPackage := ValPackage{Val: &c.ValData}
		err := newValPackage.UpdateVal(string(data))
		if err != nil {
			log.Println("Error updating val package", err)
			return
		}
		log.Println("Successfully updated val package for", c.ValData.Name)

		// The things the user can change in the yaml metadata
		valCreateReqData := valgo.NewValsCreateRequest(newValPackage.Val.GetCode())
		valCreateReqData.SetPrivacy(c.ValData.GetPrivacy())
		valCreateReqData.SetReadme(c.ValData.GetReadme())

		// Make the request to update the val
		valCreateReq := c.client.APIClient.ValsAPI.ValsCreateVersion(ctx, c.ValData.GetId()).ValsCreateRequest(*valCreateReqData)
		extVal, resp, err := valCreateReq.Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Println("Error updating val", err)
		} else {
			log.Println("Successfully updated val", c.ValData.Name)
		}

		// Update the val to the new updated val
		c.ValData = *extVal
		c.NotifyContent(0, int64(len(data)))
		c.ModifiedNow()
		log.Println("Updated val file", c.ValData.Name)
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
	log.Println("Getting attributes for val file", f.ValData.Name)

	// Set the mode to indicate a regular file with read, write, and execute
	// permissions for all
	valPackage := NewValPackage(f.client, &f.ValData)
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

	log.Println("Got attributes for val file", f.ValData.Name)
	log.Println("Size:", out.Size, "Mode:", out.Mode, "Modified:", *modified)

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
	log.Println("Setting attributes for val file", f.ValData.Name)

	out.Size = in.Size
	out.Mode = ValFileMode
	out.Atime = in.Atime
	out.Mtime = in.Mtime
	out.Ctime = in.Ctime

	return syscall.F_OK
}
