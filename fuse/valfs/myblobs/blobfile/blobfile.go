package fuse

import (
	"context"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"syscall"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const BLOB_FILE_FLAGS = syscall.S_IFREG | 0o666

// A file in the with metadata about the file and an inode
type BlobFile struct {
	fs.Inode

	BlobListing valgo.BlobListingItem
	File        *os.File
	UUID        uint64
	client      *common.Client
}

// The file handle for a blob file
type BlobFileHandle struct {
	File *os.File
}

// Create a new blob file
func NewBlobFile(data valgo.BlobListingItem, client *common.Client) *BlobFile {
	return &BlobFile{
		BlobListing: data,
		client:      client,
		UUID:        rand.Uint64(),
	}
}

// Fetch the data stored under the blob, and stream it to a tempfile. Then
// return the file descriptor for the tempfile for use as a loopback.
func (f *BlobFile) getDataFile() (*os.File, error) {
	tempFile, err := os.CreateTemp("", "valfs-blob-*")
	if err != nil {
		common.ReportError("Failed to create temp file", err)
		return nil, err
	}

	resp, err := f.client.APIClient.RawRequest("GET", "/v1/blob/"+f.BlobListing.Key, nil)
	if err != nil {
		common.ReportError("Failed to fetch blob data", err)
		return nil, err
	}

	bytesWritten, err := io.Copy(tempFile, resp.Body)
	if err != nil {
		common.ReportError("Error writing to blob file", err)
		return nil, err
	}
	bytesWritten, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		common.ReportError("Error writing to blob file", err)
		return nil, err
	}

	log.Printf("Wrote %d bytes to blobfile at %s", bytesWritten, f.BlobListing.Key)
	return tempFile, nil
}

var _ = (fs.NodeOpener)((*BlobFile)(nil))

// Get a file descriptor for a blob file
func (f *BlobFile) Open(ctx context.Context, openFlags uint32) (
	fh fs.FileHandle,
	fuseFlags uint32,
	errno syscall.Errno,
) {
	log.Println("Opening blob file", f.BlobListing.Key)

	file, err := f.getDataFile()
	if err != nil {
		return nil, 0, syscall.EIO
	}
	f.File = file
	fh = &BlobFileHandle{File: f.File}

	// Return FOPEN_DIRECT_IO so content is not cached
	return fh, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

// Read from the blob file by accessing the underlying temp file file descriptor
func (f *BlobFileHandle) Read(
	ctx context.Context,
	buf []byte,
	off int64,
) (res fuse.ReadResult, errno syscall.Errno) {
	log.Printf("Opening up %s", f.File.Name())
	r := fuse.ReadResultFd(uintptr(f.File.Fd()), off, len(buf))
	return r, syscall.F_OK
}

// Make sure the file is always read/write/executable even if changed
func (f *BlobFile) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	log.Println("Getting attributes for blob", f.BlobListing.Key)

	out.Mode = BLOB_FILE_FLAGS
	out.Size = uint64(*f.BlobListing.Size)
	modified := f.BlobListing.GetLastModified()
	out.SetTimes(&modified, &modified, &modified)

	log.Println("Size:", out.Size, "Mode:", out.Mode, "Modified:", modified)
	return syscall.F_OK
}

var _ = (fs.NodeWriter)((*BlobFile)(nil))

// Write data to a val file and the corresponding val
func (c *BlobFile) Write(
	ctx context.Context,
	uncastFh fs.FileHandle,
	data []byte,
	off int64,
) (written uint32, errno syscall.Errno) {
	fh := uncastFh.(*BlobFileHandle)
	log.Println("Writing to blob file", fh.File.Name())

	// Write to the temp file
	wrote, err := fh.File.WriteAt(data, off)
	if err != nil {
		return 0, syscall.EIO
	}
	// Post the new data
	resp, err := c.client.APIClient.BlobsAPI.BlobsStore(ctx, c.BlobListing.Key).Body(c.File).Execute()
	if err != nil {
		common.ReportError("Failed to write to blob file", err)
		return 0, syscall.EIO
	} else if resp.StatusCode != http.StatusCreated {
		common.ReportErrorResp("Failed to write to blob file, unexpected status code: %d", resp, resp.StatusCode)
		return 0, syscall.EIO
	}

	return uint32(wrote), syscall.F_OK
}

var _ = (fs.NodeSetattrer)((*BlobFile)(nil))

// Accept the request to change attrs, but ignore the new attrs, to comply with
// editors expecting to be able to change them
func (f *BlobFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	log.Println("Setting attributes for blob", f.BlobListing.Key)

	out.Size = in.Size
	out.Mode = BLOB_FILE_FLAGS
	out.Atime = in.Atime
	out.Mtime = in.Mtime
	out.Ctime = in.Ctime

	return syscall.F_OK
}
