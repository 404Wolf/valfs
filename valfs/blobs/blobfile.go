package valfs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"syscall"

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

	Meta   valgo.BlobListingItem
	Upload *BlobUpload
	blobs  *BlobsDir
}

var _ = (fs.NodeOpener)((*BlobFile)(nil))
var _ = (fs.NodeSetattrer)((*BlobFile)(nil))
var _ = (fs.NodeGetattrer)((*BlobFile)(nil))
var _ = (fs.NodeReleaser)((*BlobFile)(nil))

// BlobFileHandle represents an open file handle
type BlobFileHandle struct {
	client *common.Client
	file   *os.File
}

// NewBlobFileAuto creates a new BlobFile with a random UUID
func NewBlobFileAuto(
	data valgo.BlobListingItem,
	blobsDir *BlobsDir,
) *BlobFile {
	return NewBlobFile(data, blobsDir)
}

// NewBlobFile creates a new BlobFile with the given data, client, and UUID
func NewBlobFile(
	data valgo.BlobListingItem,
	blobsDir *BlobsDir,
) *BlobFile {
	common.Logger.Info("Creating new BlobFile with key %s", data.Key)
	blobFile := &BlobFile{
		Meta:  data,
		blobs: blobsDir,
	}
	blobFile.Upload = &BlobUpload{BlobFile: blobFile}
	return blobFile
}

// NewBlobFileHandle creates a new BlobFileHandle for the given file
func NewBlobFileHandle(file *os.File) *BlobFileHandle {
	common.Logger.Info("Creating new BlobFileHandle for %s", file.Name())
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
	common.Logger.Info("Opening blob file %s with flags %d", f.Meta.Key, openFlags)

	file, existed, err := f.EnsureTempFile()
	if err != nil {
		common.Logger.Info("Failed to open temporary file for %s: %v", f.Meta.Key, err)
		return nil, 0, syscall.EIO
	}

	// Only fetch content if the file doesn't exist or is empty
	if !existed || (openFlags&uint32(os.O_APPEND) != 0) {
		fileInfo, _ := file.Stat()
		if fileInfo.Size() == 0 {
			common.Logger.Info("Fetching content for blob %s", f.Meta.Key)
			resp, err := f.blobs.client.APIClient.RawRequest(
				ctx,
				http.MethodGet,
				"/v1/blob/"+f.Meta.Key,
				nil,
			)
			if err != nil && !os.IsNotExist(err) {
				common.Logger.Info("Failed to fetch blob content for %s: %v", f.Meta.Key, err)
				file.Close()
				f.RemoveTempFile()
				return nil, 0, syscall.EIO
			}
			if err == nil {
				defer resp.Body.Close()
				file.Seek(0, 0)
				_, err = io.Copy(file, resp.Body)
				if err != nil {
					common.Logger.Info("Failed to copy blob content to file for %s: %v", f.Meta.Key, err)
					file.Close()
					f.RemoveTempFile()
					return nil, 0, syscall.EIO
				}
			}
		}
	}

	// Set initial size if not set
	if f.Meta.Size == nil {
		size := int64(0)
		if fileInfo, err := file.Stat(); err == nil {
			size = fileInfo.Size()
		}
		f.Meta.SetSize(int32(size))
	}

	return NewBlobFileHandle(file), fuse.O_ANYWRITE, syscall.F_OK
}

// Read reads data from the file at the specified offset
func (f *BlobFileHandle) Read(
	ctx context.Context,
	buf []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	common.Logger.Info("Reading %d bytes at offset %d from %s", len(buf), off, f.file.Name())
	return fuse.ReadResultFd(uintptr(f.file.Fd()), off, len(buf)), syscall.F_OK
}

// Getattr retrieves the attributes of the blob file
func (f *BlobFile) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	common.Logger.Info("Getting attributes for blob %s", f.Meta.Key)
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
	common.Logger.Info("Setting attributes for blob %s", f.Meta.Key)
	out.Mode = BlobFileFlags
	return syscall.F_OK
}

// Release handles cleanup when the file is closed
func (f *BlobFile) Release(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	common.Logger.Info("Releasing blob file %s", f.Meta.Key)

	if f.Upload.Ongoing() {
		common.Logger.Info("Waiting for upload to finish for %s", f.Meta.Key)
		f.Upload.Finish()
	}

	common.Logger.Info("Removed temporary file %s", f.tempFilePath())
	f.RemoveTempFile()

	return syscall.F_OK
}
