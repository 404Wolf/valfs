package fuse

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"sync"
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
	BlobListing     valgo.BlobListingItem
	UUID            uint64
	client          *common.Client
	uploadMutex     sync.Mutex
	isUpdating      bool
	writePipe       *io.PipeWriter
	readPipe        *io.PipeReader
	uploadWaitGroup sync.WaitGroup
}

type BlobFileHandle struct {
	file *os.File
}

var _ = (fs.NodeOpener)((*BlobFile)(nil))
var _ = (fs.NodeWriter)((*BlobFile)(nil))
var _ = (fs.NodeSetattrer)((*BlobFile)(nil))
var _ = (fs.NodeGetattrer)((*BlobFile)(nil))
var _ = (fs.NodeReleaser)((*BlobFile)(nil))

// NewBlobFileAuto creates a new BlobFile instance and autogenerates a uuid
func NewBlobFileAuto(data valgo.BlobListingItem, client *common.Client) *BlobFile {
	return NewBlobFile(data, client, rand.Uint64())
}

// NewBlobFile creates a new BlobFile instance
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

// NewBlobFileHandle creates a new BlobFileHandle instance
func NewBlobFileHandle(file *os.File) *BlobFileHandle {
	log.Printf("Creating new BlobFileHandle for %s", file.Name())
	return &BlobFileHandle{file: file}
}

// TempFilePath returns the path for the temporary file
func (f *BlobFile) TempFilePath() string {
	// Ensure the folder exists
	os.MkdirAll("/tmp/valfs-blobs", 0755)
	return fmt.Sprintf("/tmp/valfs-blobs/blob-%d", f.UUID)
}

// Open opens the blob file and fetches its content
func (f *BlobFile) Open(
	ctx context.Context,
	openFlags uint32,
) (fs.FileHandle, uint32, syscall.Errno) {
	log.Printf("Opening blob file %s", f.BlobListing.Key)

	tempFilePath := f.TempFilePath()
	_, err := os.Stat(tempFilePath)
	fileExisted := !os.IsNotExist(err)

	// Create if not exists (because of the O_CREATE flag)
	file, err := os.OpenFile(tempFilePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Printf("Failed to open temporary file %s: %v", tempFilePath, err)
		return nil, 0, syscall.EIO
	}

	// If the file didn't exist before that means we'll need to fetch its
	// contents
	if !fileExisted {
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
			return nil, 0, syscall.EIO
		}
		defer resp.Body.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			log.Printf("Failed to copy blob content to file for %s: %v", f.BlobListing.Key, err)
			file.Close()
			return nil, 0, syscall.EIO
		}
		log.Printf("Successfully fetched and wrote content for blob %s", f.BlobListing.Key)
	}

	return &BlobFileHandle{file: file}, fuse.O_ANYWRITE, syscall.F_OK
}

// Read reads data from the blob file
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

// Write writes data to the blob file and updates the corresponding blob
func (c *BlobFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (uint32, syscall.Errno) {
	bfh := fh.(*BlobFileHandle)

	log.Printf("Writing %d bytes at offset %d to %s", len(data), off, c.BlobListing.Key)

	// Write to the temp file at the specified offset
	wrote, err := bfh.file.WriteAt(data, off)
	if err != nil {
		log.Printf("Failed to write to temporary file for %s: %v", c.BlobListing.Key, err)
		return 0, syscall.EIO
	}

	// Get the current file size
	fileInfo, err := bfh.file.Stat()
	if err != nil {
		log.Printf("Failed to stat temporary file for %s: %v", c.BlobListing.Key, err)
		return 0, syscall.EIO
	}

	// Update the size in the BlobListing
	c.BlobListing.SetSize(int32(fileInfo.Size()))

	c.uploadMutex.Lock()
	defer c.uploadMutex.Unlock()

	if !c.isUpdating {
		log.Printf("Starting new upload for %s", c.BlobListing.Key)

		c.isUpdating = true
		c.readPipe, c.writePipe = io.Pipe()

		// We won't release the file while we are uploading
		c.uploadWaitGroup.Add(1)
		go c.uploadToPipe(ctx)
	}

	// Write the data to the pipe
	_, err = c.writePipe.Write(data)
	if err != nil {
		log.Printf("Failed to write to pipe for %s: %v", c.BlobListing.Key, err)
		return 0, syscall.EIO
	}

	log.Printf("Successfully wrote %d bytes to %s", wrote, c.BlobListing.Key)
	return uint32(wrote), syscall.F_OK
}

func (c *BlobFile) uploadToPipe(ctx context.Context) {
	defer c.uploadWaitGroup.Done()
	defer c.readPipe.Close()

	log.Printf("Starting upload to server for %s", c.BlobListing.Key)

	_, err := c.client.APIClient.RawRequest(
		ctx,
		http.MethodPost,
		"/v1/blob/"+c.BlobListing.Key,
		c.readPipe,
	)
	if err != nil {
		log.Printf("Failed to upload blob content for %s: %v", c.BlobListing.Key, err)
		return
	}

	// Update the last modified time
	now := time.Now()
	c.BlobListing.SetLastModified(now)

	c.uploadMutex.Lock()
	c.isUpdating = false
	c.uploadMutex.Unlock()

	log.Printf("Finished upload to server for %s", c.BlobListing.Key)
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

// Release is called when the file is closed
func (f *BlobFile) Release(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	log.Printf("Releasing blob file %s", f.BlobListing.Key)
	f.uploadMutex.Lock()
	defer f.uploadMutex.Unlock()

	if f.isUpdating {
		log.Printf("Waiting for ongoing upload to finish for %s", f.BlobListing.Key)
		// Close the write pipe to signal the end of the upload
		f.writePipe.Close()
		// Wait for the upload to finish
		f.uploadWaitGroup.Wait()
		log.Printf("Ongoing upload finished for %s", f.BlobListing.Key)
	}

	return syscall.F_OK
}
