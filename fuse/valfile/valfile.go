package fuse

import (
	"context"
	"log"
	"net/http"
	"syscall"
	"time"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const ValFileMode = syscall.S_IFREG | 0o777

// A file in the val file system, with metadata about the file and an inode
type ValFile struct {
	fs.Inode
	ModifiedAt   time.Time
	BasicData    valgo.BasicVal
	ExtendedData valgo.ExtendedVal
	client       *common.Client
}

func getValVersionCreatedAt(val valgo.ExtendedVal, client *common.Client) *time.Time {
	modified := val.VersionCreatedAt
	if modified == nil {
		ctx := context.Background()
		versionList, resp, err := client.APIClient.ValsAPI.ValsList(ctx, val.Id).Offset(0).Limit(1).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Println("Error fetching version list", err)
		}
		modified = &versionList.Data[0].CreatedAt
	}
	return modified
}

// Create a new val file, but does not attach an inode embedding
func NewValFileLazyFetchExtended(
	ctx context.Context,
	val valgo.BasicVal,
	client *common.Client,
) (*ValFile, error) {
	log.Println("Create new val file named", val.Name)

	// Create a val file and get a reference
	valFile := &ValFile{
		BasicData:  val,
		client:     client,
		ModifiedAt: time.Now(),
	}

	// Get the last modified date to cache
	go func() {
		extVal, resp, err := client.APIClient.ValsAPI.ValsGet(ctx, val.Id).Execute()

		if err != nil || resp.StatusCode != http.StatusOK {
			log.Println("Error fetching val", err)
			return
		}

		valFile.ModifiedAt = *getValVersionCreatedAt(*extVal, client)
		log.Println("Setting new val file modified at to", valFile.ModifiedAt)
	}()

	// Return the val file as is now, it will get populated with extended val later
	return valFile, nil
}

// Create a new val file already knowing what the extended val is
func NewValFileFromExtended(
	val valgo.ExtendedVal,
	client *common.Client,
) (*ValFile, error) {
	log.Println("Create new val file named", val.Name)

	return &ValFile{
		ExtendedData: val,
		client:       client,
		ModifiedAt:   *getValVersionCreatedAt(val, client),
	}, nil
}

// A file handle that carries separate content for each open call
type ValFileHandle struct {
	ValFile *ValFile
	client  *common.Client
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
	log.Println("Opening val file", f.ExtendedData.Name)

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
	log.Println("Reading val file", fh.ValFile.ExtendedData.Name)

	// Provide the Val's code as the data
	valPackage := NewValPackage(fh.client, &fh.ValFile.ExtendedData)
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
	log.Println("Writing to val file", c.ExtendedData.Name)

	// Create new packed file contents
	newValPackage := ValPackage{Val: &c.ExtendedData}
	err := newValPackage.UpdateVal(string(data))
	if err != nil {
		log.Println("Error updating val package", err)
		return 0, syscall.EIO
	}
	log.Println("Successfully updated val package for", c.ExtendedData.Name)

	// The things the user can change in the yaml metadata
	valCreateReqData := valgo.NewValsCreateRequest(newValPackage.Val.GetCode())
	valCreateReqData.SetPrivacy(c.ExtendedData.GetPrivacy())
	valCreateReqData.SetReadme(c.ExtendedData.GetReadme())

	// Make the request to update the val
	valCreateReq := c.client.APIClient.ValsAPI.ValsCreateVersion(ctx, c.ExtendedData.GetId()).ValsCreateRequest(*valCreateReqData)
	extVal, resp, err := valCreateReq.Execute()
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Println("Error updating val", err)
		return 0, syscall.EIO
	}
	log.Println("Successfully updated val", c.ExtendedData.Name)

	// Update the val to the new updated val
	c.ExtendedData = *extVal
	c.NotifyContent(0, int64(len(data)))
	c.ModifiedNow()
	log.Println("Updated val file", c.ExtendedData.Name)

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
	log.Println("Getting attributes for val file", f.ExtendedData.Name)

	// Set the mode to indicate a regular file with read, write, and execute
	// permissions for all
	valPackage := NewValPackage(f.client, &f.ExtendedData)
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

	log.Println("Got attributes for val file", f.ExtendedData.Name)
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
	log.Println("Setting attributes for val file", f.ExtendedData.Name)

	out.Size = in.Size
	out.Mode = ValFileMode
	out.Atime = in.Atime
	out.Mtime = in.Mtime
	out.Ctime = in.Ctime

	return syscall.F_OK
}
