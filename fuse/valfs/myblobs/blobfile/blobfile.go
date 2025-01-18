package fuse

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const BlobFileFlags = syscall.S_IFREG | 0o666

type BlobFile struct {
	fs.Inode
	BlobListing valgo.BlobListingItem
	UUID        uint64
	client      *common.Client
}

type BlobFileHandle struct {
	File *os.File
}

var _ = (fs.NodeOpener)((*BlobFile)(nil))
var _ = (fs.NodeWriter)((*BlobFile)(nil))
var _ = (fs.NodeSetattrer)((*BlobFile)(nil))

// NewBlobFile creates a new BlobFile instance
func NewBlobFile(data valgo.BlobListingItem, client *common.Client) *BlobFile {
	return &BlobFile{
		BlobListing: data,
		client:      client,
		UUID:        rand.Uint64(),
	}
}

// getTempFilePath returns the path for the temporary file
func (f *BlobFile) getTempFilePath() string {
	return fmt.Sprintf("/tmp/valfs-blob-%d", f.UUID)
}

// Open opens the blob file and fetches its content
func (f *BlobFile) Open(
	ctx context.Context,
	openFlags uint32,
) (fs.FileHandle, uint32, syscall.Errno) {
	log.Printf("Opening blob file %s", f.BlobListing.Key)

	tempFilePath := f.getTempFilePath()
	_, err := os.Stat(tempFilePath)
	fileExisted := os.IsNotExist(err)

	// Create if not exists (because of the O_CREATE flag)
	file, err := os.OpenFile(tempFilePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		common.ReportError("Failed to open temporary file", err)
		return nil, 0, syscall.EIO
	}


  // If the file didn't exist before that means we'll need to fetch its
  // contents
	if !fileExisted {
		resp, err := f.client.APIClient.RawRequest("GET", "/v1/blob/"+f.BlobListing.Key, nil)
		if err != nil {
			common.ReportError("Failed to fetch blob content", err)
			file.Close()
			return nil, 0, syscall.EIO
		}
		defer resp.Body.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			common.ReportError("Failed to copy blob content to file", err)
			file.Close()
			return nil, 0, syscall.EIO
		}
	}

	return &BlobFileHandle{File: file}, fuse.O_ANYWRITE, syscall.F_OK
}

// Read reads data from the blob file
func (f *BlobFileHandle) Read(
	ctx context.Context,
	buf []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	return fuse.ReadResultFd(uintptr(f.File.Fd()), off, len(buf)), syscall.F_OK
}

// Getattr retrieves the attributes of the blob file
func (f *BlobFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = BlobFileFlags
	out.Size = uint64(*f.BlobListing.Size)
	modified := f.BlobListing.GetLastModified()
	out.SetTimes(&modified, &modified, &modified)
	return syscall.F_OK
}

// Write writes data to the blob file and updates the corresponding blob
func (c *BlobFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (uint32, syscall.Errno) {
	bfh := fh.(*BlobFileHandle)

	// Write to the temp file at the specified offset
	wrote, err := bfh.File.WriteAt(data, off)
	if err != nil {
		common.ReportError("Failed to write to temporary file", err)
		return 0, syscall.EIO
	}

	// Get the current file size
	fileInfo, err := bfh.File.Stat()
	if err != nil {
		common.ReportError("Failed to stat temporary file", err)
		return 0, syscall.EIO
	}
	fileSize := fileInfo.Size()

	// Update the size in the BlobListing
	int32Size := int32(fileSize)
	c.BlobListing.Size = &int32Size

	// Seek to the start of the file
	if _, err := bfh.File.Seek(0, 0); err != nil {
		common.ReportError("Failed to seek to start of file", err)
		return 0, syscall.EIO
	}

	// Upload the entire file
	_, err = c.client.APIClient.RawRequest(http.MethodPost, "/v1/blob/"+c.BlobListing.Key, bfh.File)
	if err != nil {
		common.ReportError("Failed to upload blob content", err)
		return 0, syscall.EIO
	}

	// Update the last modified time (this might not be the same as we would see
	// with the API, but it makes editors happy)
	c.BlobListing.SetLastModified(time.Now())

	return uint32(wrote), syscall.F_OK
}

// Setattr sets the attributes of the blob file
func (f *BlobFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	out.Size = in.Size
	out.Mode = BlobFileFlags
	modified := f.BlobListing.GetLastModified()
	out.SetTimes(&modified, &modified, &modified)
	return syscall.F_OK
}
