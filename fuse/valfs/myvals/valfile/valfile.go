package fuse

import (
	"context"
	"errors"
	"log"
	"syscall"
	"time"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const VAL_FILE_FLAGS = syscall.S_IFREG | 0o777

// A file in the val file system, with metadata about the file and an inode
type ValFile struct {
	fs.Inode

	ModifiedAt   time.Time
	BasicData    valgo.BasicVal
	ExtendedData *valgo.ExtendedVal
	client       *common.Client
}

var _ = (fs.NodeSetattrer)((*ValFile)(nil))
var _ = (fs.NodeGetattrer)((*ValFile)(nil))
var _ = (fs.NodeWriter)((*ValFile)(nil))
var _ = (fs.NodeOpener)((*ValFile)(nil))
var _ = (fs.FileReader)((*ValFileHandle)(nil))

// Get the extended val data for the val. If it is already a member of the
// valfile then retreive it from cache. If it has not been fetched yet then
// fetch it.
func (f *ValFile) GetExtendedData(ctx context.Context) (*valgo.ExtendedVal, error) {
	if f.ExtendedData == nil {
		extVal, _, err := f.client.APIClient.ValsAPI.ValsGet(ctx, f.BasicData.GetId()).Execute()
		if err != nil {
			return nil, errors.New("Failed to fetch extended data")
		}
		f.ExtendedData = extVal
	}
	return f.ExtendedData, nil
}

// Get the date at which the last version of the underlying val was created at
func getValVersionCreatedAt(val valgo.ExtendedVal, client *common.Client) *time.Time {
	modified := val.VersionCreatedAt
	if modified == nil {
		ctx := context.Background()
		versionList, _, err := client.APIClient.ValsAPI.ValsList(ctx,
			val.Id).Offset(0).Limit(1).Execute()
		if err != nil {
			common.ReportError("Error fetching version list", err)
		}
		modified = &versionList.Data[0].CreatedAt
	}
	return modified
}

// Reading val files requires having access to the extended val file with all
// of the metadata since metadata is placed at the top of files in frontmatter
// yaml. However, just basic vald data (which you get from listing your vals,
// and other bulk actions) is sufficient for some operations like listing vals.
// To conserve memory and reduce unncessary API requests, if we create a val
// file from a basic val we will automatically fetch the extended val when a
// file handle is requested, instead of at the time of adding the inode.
func NewValFileFromBasicVal(
	ctx context.Context,
	val valgo.BasicVal,
	client *common.Client,
) (*ValFile, error) {
	log.Println("Create new val file named", val.Name, "from basic val")

	// Create a val file and get a reference
	valFile := &ValFile{
		BasicData:  val,
		client:     client,
		ModifiedAt: time.Now(), // For now say that
	}

	// Return the val file as is now, it will get populated with extended val
	// later as needed
	return valFile, nil
}

// Add a new val file with prefetched extended val data. When a file handle is
// requested the data is already present in the val file struct.
func NewValFileFromExtendedVal(
	val valgo.ExtendedVal,
	client *common.Client,
) (*ValFile, error) {
	log.Println("Create new val file named", val.Name, "from extended val")

	return &ValFile{
		ExtendedData: &val,
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

// Get a file descriptor for a val file
func (f *ValFile) Open(ctx context.Context, openFlags uint32) (
	fh fs.FileHandle,
	fuseFlags uint32,
	errno syscall.Errno,
) {
	if f.ExtendedData == nil {
		log.Println("Valfile was lazy. Now getting extended val data for", f.BasicData.Name)
		extVal, _, err := f.client.APIClient.ValsAPI.ValsGet(ctx, f.BasicData.GetId()).Execute()
		if err != nil {
			log.Println("Error fetching val", err)
			return nil, 0, syscall.EIO
		}
		f.ExtendedData = extVal
	}
	log.Println("Opening val file", f.BasicData.Name)

	// Create a new file handle for the val
	fh = &ValFileHandle{
		ValFile: f,
		client:  f.client,
	}

	// Return FOPEN_DIRECT_IO so content is not cached
	return fh, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

// Provide the content of the val as the content of the file
func (fh *ValFileHandle) Read(
	ctx context.Context,
	dest []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	// Provide the Val's code as the data
	extVal, err := fh.ValFile.GetExtendedData(ctx)
	if err != nil {
		return nil, syscall.EIO
	}
	log.Println("Reading val file", extVal)

	valPackage := NewValPackage(fh.client, extVal)
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

// Write data to a val file and the corresponding val
func (c *ValFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (written uint32, errno syscall.Errno) {
	prevExtVal, err := c.GetExtendedData(ctx)
	log.Println("Writing to val file", prevExtVal)

	// Create new packed file contents
	newValPackage := ValPackage{Val: prevExtVal}
	if err != nil {
		log.Println("Error updating val package", err)
		return 0, syscall.EIO
	}
	newValPackage.UpdateVal(string(data))
	extVal := newValPackage.Val

	// The things the user can change in the yaml metadata
	valCreateReqData := valgo.NewValsCreateRequest(newValPackage.Val.GetCode())

	// Val town requires at least one character
	if len(valCreateReqData.Code) == 0 {
		valCreateReqData.Code = " "
	}

	valCreateReqData.SetPrivacy(prevExtVal.GetPrivacy())
	valCreateReqData.SetReadme(prevExtVal.GetReadme())
	log.Printf("New val data %v", valCreateReqData)

	// Make the request to update the val
	valCreateReq := c.client.APIClient.ValsAPI.ValsCreateVersion(ctx, prevExtVal.GetId()).ValsCreateRequest(*valCreateReqData)
	_, _, err = valCreateReq.Execute()
	if err != nil {
		log.Println("Error updating val", err)
		return 0, syscall.EIO
	}
	log.Println("Successfully updated val", prevExtVal.Name)

	// Because of a bug in val town we also need to "update" the val
	valUpdateReqData := valgo.NewValsUpdateRequestWithDefaults()
	valUpdateReqData.SetPrivacy(extVal.GetPrivacy())
	valUpdateReqData.SetReadme(extVal.GetReadme())
	log.Printf("New val data %v", valUpdateReqData)
	_, err = c.client.APIClient.ValsAPI.ValsUpdate(ctx, extVal.GetId()).ValsUpdateRequest(*valUpdateReqData).Execute()
	if err != nil {
		log.Println("Error updating val", err)
		return 0, syscall.EIO
	}
	log.Println("Updated val file", prevExtVal.Name)

	// And finally, retreive the val's extended data again
	extVal, _, err = c.client.APIClient.ValsAPI.ValsGet(ctx, prevExtVal.GetId()).Execute()
	if err != nil {
		common.ReportError("Error fetching val", err)
		return 0, syscall.EIO
	}
	c.ExtendedData = extVal

	c.ModifiedNow()
	return uint32(len(data)), syscall.F_OK
}

// Make sure the file is always read/write/executable even if changed
func (f *ValFile) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	log.Println("Getting attributes for val file", f.BasicData.Name)

	// Do not fetch extended data if we haven't already, just use placeholder!
	if f.ExtendedData == nil {
		out.Mode = VAL_FILE_FLAGS
		// The size of the actual code, plus a bit extra
		// TODO: Figure out what the actual maximum amount of "extra" is
		out.Size = uint64(len(f.BasicData.GetCode()) + 500)
		modified := time.Unix(0, 0)
		out.SetTimes(&modified, &modified, &modified)
		return 0
	}

	valPackage := NewValPackage(f.client, f.ExtendedData)
	contentLen, err := valPackage.Len()
	if err != nil {
		log.Println("Error getting content length", err)
		return syscall.EIO
	}

	out.Size = uint64(contentLen)
	out.Mode = VAL_FILE_FLAGS

	// Set timestamps to be modified now
	modified := &f.ModifiedAt
	out.SetTimes(modified, modified, modified)

	log.Println("Got attributes for val file", f.BasicData.Name)
	log.Println("Size:", out.Size, "Mode:", out.Mode, "Modified:", *modified)

	return syscall.F_OK
}

// Accept the request to change attrs, but ignore the new attrs, to comply with
// editors expecting to be able to change them
func (f *ValFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	log.Println("Setting attributes for val file", f.BasicData.Name)

	out.Size = in.Size
	out.Mode = VAL_FILE_FLAGS
	out.Atime = in.Atime
	out.Mtime = in.Mtime
	out.Ctime = in.Ctime

	return syscall.F_OK
}
