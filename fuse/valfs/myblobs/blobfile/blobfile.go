package fuse

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// BlobFileFlags defines the file permissions for blob files
const BlobFileFlags = syscall.S_IFREG | 0o666

// BlobFile represents a file in the FUSE filesystem
type BlobFile struct {
	fs.Inode
	BlobListing            valgo.BlobListingItem
	UUID                   uint64
	client                 *common.Client
	uploadCtx              context.Context
	uploadCancel           context.CancelFunc
	uploadInProgress       bool
	dataLenInAir           int64
	failedLastWrite        bool
	writePipe              *io.PipeWriter
	readPipe               *io.PipeReader
	writePipeLastByteIndex int64
	writeTimer             *time.Timer // Timer to track write inactivity
}

// BlobFileHandle represents an open file handle
type BlobFileHandle struct {
	file *os.File
}

// Ensure BlobFile implements necessary interfaces
var _ = (fs.NodeOpener)((*BlobFile)(nil))
var _ = (fs.NodeWriter)((*BlobFile)(nil))
var _ = (fs.NodeSetattrer)((*BlobFile)(nil))
var _ = (fs.NodeGetattrer)((*BlobFile)(nil))
var _ = (fs.NodeReleaser)((*BlobFile)(nil))

// writeTimeout defines the duration of inactivity after which the write pipe will be closed
const writeTimeout = 5 * time.Second

// NewBlobFileAuto creates a new BlobFile with a random UUID
func NewBlobFileAuto(data valgo.BlobListingItem, client *common.Client) *BlobFile {
	return NewBlobFile(data, client, rand.Uint64())
}

// NewBlobFile creates a new BlobFile with the given data, client, and UUID
func NewBlobFile(
	data valgo.BlobListingItem,
	client *common.Client,
	uuid uint64,
) *BlobFile {
	log.Printf("Creating new BlobFile for %s with UUID %d", data.Key, uuid)
	return &BlobFile{
		BlobListing: data,
		client:      client,
		UUID:        uuid,
	}
}

// NewBlobFileHandle creates a new BlobFileHandle for the given file
func NewBlobFileHandle(file *os.File) *BlobFileHandle {
	log.Printf("Creating new BlobFileHandle for %s", file.Name())
	return &BlobFileHandle{file: file}
}

// TempFilePath returns the path for the temporary file associated with this BlobFile
func (f *BlobFile) TempFilePath() string {
	os.MkdirAll("/tmp/valfs-blobs", 0755)
	return fmt.Sprintf("/tmp/valfs-blobs/blob-%d", f.UUID)
}

// Open opens the blob file and fetches its content from the server
func (f *BlobFile) Open(
	ctx context.Context,
	openFlags uint32,
) (fs.FileHandle, uint32, syscall.Errno) {
	log.Printf("Opening blob file %s", f.BlobListing.Key)

	tempFilePath := f.TempFilePath()

	// Open or create the temporary file
	file, err := os.OpenFile(tempFilePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Printf("Failed to open temporary file for %s: %v", f.BlobListing.Key, err)
		return nil, 0, syscall.EIO
	}

	// Fetch the blob content from the server
	log.Printf("Fetching content for blob %s", f.BlobListing.Key)
	resp, err := f.client.APIClient.RawRequest(
		ctx,
		http.MethodGet,
		"/v1/blob/"+f.BlobListing.Key,
		nil,
	)
	if err != nil {
		log.Printf("Failed to fetch blob content for %s: %v", f.BlobListing.Key, err)
		file.Close()
		os.Remove(tempFilePath)
		return nil, 0, syscall.EIO
	}
	defer resp.Body.Close()

	// Copy the content to the temporary file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		log.Printf("Failed to copy blob content to file for %s: %v", f.BlobListing.Key, err)
		file.Close()
		os.Remove(tempFilePath)
		return nil, 0, syscall.EIO
	}
	log.Printf("Successfully fetched and wrote content for blob %s", f.BlobListing.Key)

	return &BlobFileHandle{file: file}, fuse.O_ANYWRITE, syscall.F_OK
}

// Read reads data from the file at the specified offset
func (f *BlobFileHandle) Read(
	ctx context.Context,
	buf []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	log.Printf("Reading %d bytes at offset %d from %s", len(buf), off, f.file.Name())
	return fuse.ReadResultFd(uintptr(f.file.Fd()), off, len(buf)), syscall.F_OK
}

// Getattr retrieves the attributes of the blob file
func (f *BlobFile) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	log.Printf("Getting attributes for blob %s", f.BlobListing.Key)
	out.Mode = BlobFileFlags
	out.Size = uint64(*f.BlobListing.Size)
	modified := f.BlobListing.GetLastModified()
	out.SetTimes(&modified, &modified, &modified)
	return syscall.F_OK
}

// Setattr sets the attributes of the blob file
func (f *BlobFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	log.Printf("Setting attributes for blob %s", f.BlobListing.Key)
	out.Size = in.Size
	out.Mode = BlobFileFlags
	return syscall.F_OK
}

// Release closes the file handle and cleans up resources
func (f *BlobFile) Release(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	log.Printf("Releasing blob file %s", f.BlobListing.Key)

	// Wait for the upload to finish
	log.Printf("Waiting for upload to finish for %s", f.BlobListing.Key)
	for f.uploadInProgress {
		time.Sleep(100 * time.Millisecond)
	}

	// Stop the write timeout timer
	if f.writeTimer != nil {
		f.writeTimer.Stop()
	}

	// Close and remove the temporary file
	bfh := fh.(*BlobFileHandle)
	tempFilePath := bfh.file.Name()
	bfh.file.Close()
	err := os.Remove(tempFilePath)
	if err != nil {
		log.Printf("Failed to remove temporary file %s: %v", tempFilePath, err)
	} else {
		log.Printf("Removed temporary file %s", tempFilePath)
	}

	return syscall.F_OK
}
