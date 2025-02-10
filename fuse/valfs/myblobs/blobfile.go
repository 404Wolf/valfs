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

	Meta    valgo.BlobListingItem
	Upload  *BlobUpload
	myBlobs *MyBlobs
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
	blobFile := &BlobFile{
		Meta:    data,
		myBlobs: myBlobs,
	}
	blobFile.Upload = &BlobUpload{BlobFile: blobFile}
	return blobFile
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
	return fmt.Sprintf("/tmp/valfs-blobs/blob-%s", f.Meta.Key)
}

// RemoveTempFile closes and removes the temporary file
func (f *BlobFile) RemoveTempFile() error {
	tempFilePath := f.tempFilePath()
	return os.Remove(tempFilePath)
}

// EnsureTempFile opens a temporary file for read and write operations
func (f *BlobFile) EnsureTempFile() (*os.File, bool, error) {
	tempFilePath := f.tempFilePath()
	existed := true

	_, err := os.Stat(tempFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			existed = false
		} else {
			return nil, false, fmt.Errorf("error checking file status: %w", err)
		}
	}

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
	log.Printf("Opening blob file %s", f.Meta.Key)

	file, existed, err := f.EnsureTempFile()
	if err != nil {
		log.Printf("Failed to open temporary file for %s: %v", f.Meta.Key, err)
		return nil, 0, syscall.EIO
	}

	if !existed {
		log.Printf("Fetching content for blob %s", f.Meta.Key)
		resp, err := f.myBlobs.client.APIClient.RawRequest(
			ctx,
			http.MethodGet,
			"/v1/blob/"+f.Meta.Key,
			nil,
		)
		if err != nil {
			log.Printf("Failed to fetch blob content for %s: %v", f.Meta.Key, err)
			file.Close()
			f.RemoveTempFile()
			return nil, 0, syscall.EIO
		}
		defer resp.Body.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			log.Printf("Failed to copy blob content to file for %s: %v", f.Meta.Key, err)
			file.Close()
			f.RemoveTempFile()
			return nil, 0, syscall.EIO
		}
		log.Printf("Successfully fetched and wrote content for blob %s", f.Meta.Key)
	}

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
	log.Printf("Getting attributes for blob %s", f.Meta.Key)
	out.Mode = BlobFileFlags
	out.Size = uint64(*f.Meta.Size)
	modified := f.Meta.GetLastModified()
	out.SetTimes(&modified, &modified, &modified)
	return syscall.F_OK
}

// Setattr handles attribute changes
func (f *BlobFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	log.Printf("Setting attributes for blob %s", f.Meta.Key)
	out.Mode = BlobFileFlags
	return syscall.F_OK
}

// Write handles writing data to the file and managing uploads
func (f *BlobFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (uint32, syscall.Errno) {
	bfh := fh.(*BlobFileHandle)

	// Write to temporary file
	wrote, err := bfh.file.WriteAt(data, off)
	if err != nil {
		common.ReportError("Failed to write to temporary file for %s: %v", err, f.Meta.Key)
		return 0, syscall.EIO
	}
	log.Printf("Wrote %d bytes at offset %d to %s", wrote, off, f.Meta.Key)

	// Handle upload
	if !f.Upload.Ongoing() {
		if err := f.Upload.Start(); err != nil {
			common.ReportError("Failed to start upload", err)
			return 0, syscall.EIO
		}
	}

	// Write to upload
	if err := f.Upload.Write(off, data); err != nil {
		common.ReportError("Failed to write to upload", err)
		return 0, syscall.EIO
	}

	// Update metadata
	fileInfo, err := bfh.file.Stat()
	if err != nil {
		log.Printf("Failed to stat temporary file for %s: %v", f.Meta.Key, err)
		return 0, syscall.EIO
	}
	f.Meta.SetSize(fileInfo.Size())
	f.Meta.SetLastModified(time.Now())

	log.Printf("Successfully wrote %d bytes to %s", wrote, f.Meta.Key)
	return uint32(wrote), syscall.F_OK
}

// Release handles cleanup when the file is closed
func (f *BlobFile) Release(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	log.Printf("Releasing blob file %s", f.Meta.Key)

	if f.Upload != nil && f.Upload.Ongoing() {
		log.Printf("Waiting for upload to finish for %s", f.Meta.Key)
		f.Upload.Finish()
	}

	log.Printf("Removed temporary file %s", f.tempFilePath())
	f.RemoveTempFile()

	return syscall.F_OK
}
