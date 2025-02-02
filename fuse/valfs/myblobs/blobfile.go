package fuse

import (
	"context"
	"fmt"
	"io"
	"log"
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

// BlobTempDirPerms defines the permissions for the temporary directory for
// cached blobs currently in use
const BlobTempDirPerms = 0o755

// BlobFile represents a file in the FUSE filesystem
type BlobFile struct {
	fs.Inode

	BlobListing valgo.BlobListingItem
	myBlobs     *MyBlobs
}

var _ = (fs.NodeOpener)((*BlobFile)(nil))
var _ = (fs.NodeWriter)((*BlobFile)(nil))
var _ = (fs.NodeSetattrer)((*BlobFile)(nil))
var _ = (fs.NodeGetattrer)((*BlobFile)(nil))
var _ = (fs.NodeReleaser)((*BlobFile)(nil))

// BlobFileHandle represents an open file handle
type BlobFileHandle struct {
	file *os.File
}

// NewBlobFileAuto creates a new BlobFile with a random UUID
func NewBlobFileAuto(
	data valgo.BlobListingItem,
	myBlobs *MyBlobs,
) *BlobFile {
	return NewBlobFile(data, myBlobs)
}

// NewBlobFile creates a new BlobFile with the given data, client, and UUID
func NewBlobFile(
	data valgo.BlobListingItem,
	myBlobs *MyBlobs,
) *BlobFile {
	log.Printf("Creating new BlobFile with key %s", data.Key)
	return &BlobFile{
		BlobListing: data,
		myBlobs:     myBlobs,
	}
}

// NewBlobFileHandle creates a new BlobFileHandle for the given file
func NewBlobFileHandle(file *os.File) *BlobFileHandle {
	log.Printf("Creating new BlobFileHandle for %s", file.Name())
	return &BlobFileHandle{file: file}
}

// TempFilePath returns the path for the temporary file associated with this
// BlobFile
func (f *BlobFile) tempFilePath() string {
	os.MkdirAll("/tmp/valfs-blobs", BlobTempDirPerms)
	return fmt.Sprintf("/tmp/valfs-blobs/blob-%s", f.BlobListing.Key)
}

// Close and remove the temporary file associated with the given blob file
func (f *BlobFile) RemoveTempFile() error {
	tempFilePath := f.tempFilePath()
	return os.Remove(tempFilePath)
}

// OpenTempFile opens a temporary file for read and write operations.
// It returns a pointer to the opened file, a boolean indicating whether the file existed before opening,
// and an error if any occurred during the process.
func (f *BlobFile) EnsureTempFile() (*os.File, bool, error) {
	tempFilePath := f.tempFilePath()
	existed := true

	// Check if the file exists
	_, err := os.Stat(tempFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, set existed to false
			existed = false
		} else {
			// An error occurred other than file not existing
			return nil, false, fmt.Errorf("error checking file status: %w", err)
		}
	}

	// Open the file with read-write permissions, create if it doesn't exist
	file, err := os.OpenFile(tempFilePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, existed, fmt.Errorf("error opening file: %w", err)
	}

	return file, existed, nil
}

// Open opens the blob file and fetches its content from the server
func (f *BlobFile) Open(
	ctx context.Context,
	openFlags uint32,
) (fs.FileHandle, uint32, syscall.Errno) {
	// Log the attempt to open the blob file
	log.Printf("Opening blob file %s", f.BlobListing.Key)

	// Ensure a temporary file exists for this blob
	file, existed, err := f.EnsureTempFile()
	if err != nil {
		log.Printf("Failed to open temporary file for %s: %v", f.BlobListing.Key, err)
		return nil, 0, syscall.EIO
	}

	// Check if the file was created before this run of valfs
	if !existed {
		// Fetch the blob content from the server
		log.Printf("Fetching content for blob %s", f.BlobListing.Key)
		resp, err := f.myBlobs.client.APIClient.RawRequest(
			ctx,
			http.MethodGet,
			"/v1/blob/"+f.BlobListing.Key,
			nil,
		)
		if err != nil {
			log.Printf("Failed to fetch blob content for %s: %v", f.BlobListing.Key, err)
			file.Close()
			f.RemoveTempFile()
			return nil, 0, syscall.EIO
		}
		defer resp.Body.Close()

		// Copy the content to the temporary file
		_, err = io.Copy(file, resp.Body)
		if err != nil {
			log.Printf("Failed to copy blob content to file for %s: %v", f.BlobListing.Key, err)
			file.Close()
			f.RemoveTempFile()
			return nil, 0, syscall.EIO
		}
		log.Printf("Successfully fetched and wrote content for blob %s", f.BlobListing.Key)
	}

	// Return the file handle
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

// Stub for nwo
func (f *BlobFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	log.Printf("Setting attributes for blob %s", f.BlobListing.Key)

	out.Mode = BlobFileFlags

	return syscall.F_OK
}

// Write writes data to the file at the specified offset. This makes a change
// to the file, and then uploads the entire file. TODO, add piping so if you
// write directly after a previous write that is still being uploaded, this
// gets added to the queue for that socket.
func (f *BlobFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (uint32, syscall.Errno) {
	bfh := fh.(*BlobFileHandle)
	log.Printf("Writing %d bytes at offset %d to %s", len(data), off, f.BlobListing.Key)

	// Step 1: Write data to the temporary file
	wrote, err := bfh.file.WriteAt(data, off)
	if err != nil {
		common.ReportError("Failed to write to temporary file for %s: %v", err, f.BlobListing.Key)
		return 0, syscall.EIO
	}
	log.Printf("Wrote %d bytes at offset %d to %s", wrote, off, f.BlobListing.Key)

	// Step 2: Upload the entire file content
	if err := f.uploadEntireFile(ctx, bfh.file); err != nil {
		common.ReportError("Failed to upload file for %s: %v", err, f.BlobListing.Key)
		return 0, syscall.EIO
	}

	// Step 3: Update file size and last modified time
	fileInfo, err := bfh.file.Stat()
	if err != nil {
		log.Printf("Failed to stat temporary file for %s: %v", f.BlobListing.Key, err)
		return 0, syscall.EIO
	}
	f.BlobListing.SetSize(fileInfo.Size())
	f.BlobListing.SetLastModified(time.Now())

	log.Printf("Successfully wrote and uploaded %d bytes to %s", wrote, f.BlobListing.Key)

	return uint32(wrote), syscall.F_OK
}

func (f *BlobFile) uploadEntireFile(ctx context.Context, file *os.File) error {
	// Create a new file handle for uploading
	uploadFile, err := os.Open(file.Name())
	if err != nil {
		return fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer uploadFile.Close()

	// Upload the file to the server
	log.Printf("Uploading entire blob %s to server", f.BlobListing.Key)
	_, err = f.myBlobs.client.APIClient.RawRequest(
		ctx,
		http.MethodPost,
		"/v1/blob/"+f.BlobListing.Key,
		uploadFile,
	)

	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	log.Printf("Successfully uploaded entire file %s", f.BlobListing.Key)
	return nil
}

// Release closes the file handle and cleans up resources
func (f *BlobFile) Release(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	log.Printf("Releasing blob file %s", f.BlobListing.Key)

	// Wait for the upload to finish
	log.Printf("Waiting for upload to finish for %s", f.BlobListing.Key)
	f.RemoveTempFile()
	log.Printf("Removed temporary file %s", f.tempFilePath())

	return syscall.F_OK
}
