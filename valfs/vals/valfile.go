package valfs

import (
	"context"
	"errors"
	"syscall"
	"time"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const VAL_FILE_FLAGS = syscall.S_IFREG | 0o777

type ValFile struct {
	fs.Inode

	ModifiedAt   time.Time
	BasicData    valgo.BasicVal
	ExtendedData *valgo.ExtendedVal
	client       *common.Client
	parent       ValsContainer
}

var _ = (fs.NodeSetattrer)((*ValFile)(nil))
var _ = (fs.NodeGetattrer)((*ValFile)(nil))
var _ = (fs.NodeWriter)((*ValFile)(nil))
var _ = (fs.NodeOpener)((*ValFile)(nil))
var _ = (fs.FileReader)((*ValFileHandle)(nil))

func (f *ValFile) GetExtendedData(ctx context.Context) (*valgo.ExtendedVal, error) {
	if f.ExtendedData == nil {
		extVal, err := f.parent.GetValOps().Read(ctx, f.BasicData.GetId())
		if err != nil {
			return nil, errors.New("Failed to fetch extended data")
		}
		f.ExtendedData = extVal
	}
	return f.ExtendedData, nil
}

func getValVersionCreatedAt(val valgo.ExtendedVal, client *common.Client) *time.Time {
	modified := val.VersionCreatedAt
	if modified == nil {
		ctx := context.Background()
		versionList, _, err := client.APIClient.ValsAPI.ValsList(ctx,
			val.Id).Offset(0).Limit(1).Execute()
		if err != nil {
			common.Logger.Error("Error fetching version list", err)
		}
		modified = &versionList.Data[0].CreatedAt
	}
	return modified
}

func NewValFileFromBasicVal(
	ctx context.Context,
	val valgo.BasicVal,
	client *common.Client,
	parent ValsContainer,
) (*ValFile, error) {
	common.Logger.Info("Create new val file from basic val", "name", val.Name)

	valFile := &ValFile{
		BasicData:  val,
		client:     client,
		parent:     parent,
		ModifiedAt: time.Now(),
	}

	return valFile, nil
}

func NewValFileFromExtendedVal(
	val valgo.ExtendedVal,
	client *common.Client,
	parent ValsContainer,
) (*ValFile, error) {
	common.Logger.Info("Create new val file from extended val", "name", val.Name)

	return &ValFile{
		BasicData:    val.ToBasicVal(),
		ExtendedData: &val,
		client:       client,
		parent:       parent,
		ModifiedAt:   *getValVersionCreatedAt(val, client),
	}, nil
}

type ValFileHandle struct {
	ValFile *ValFile
	client  *common.Client
}

func (f *ValFile) ModifiedNow() {
	f.ModifiedAt = time.Now()
}

func (f *ValFile) Open(ctx context.Context, openFlags uint32) (
	fh fs.FileHandle,
	fuseFlags uint32,
	errno syscall.Errno,
) {
	if f.ExtendedData == nil {
		common.Logger.Info("Valfile was lazy. Now getting extended val data", "name", f.BasicData.Name)
		extVal, err := f.parent.GetValOps().Read(ctx, f.BasicData.Id)
		if err != nil {
			common.Logger.Error("Error fetching val", "error", err)
			return nil, 0, syscall.EIO
		}
		f.ExtendedData = extVal
	}
	common.Logger.Info("Opening val file", "name", f.BasicData.Name)

	fh = &ValFileHandle{
		ValFile: f,
		client:  f.client,
	}

	filename := ConstructFilename(f.BasicData.GetName(), ValType(f.BasicData.GetType()))
	waitThenMaybeDenoCache(filename, f.client)

	return fh, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

func (fh *ValFileHandle) Read(
	ctx context.Context,
	dest []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	extVal, err := fh.ValFile.GetExtendedData(ctx)
	if err != nil {
		return nil, syscall.EIO
	}
	common.Logger.Info("Reading val file", "val", extVal)

	valPackage := NewValPackage(fh.client, extVal)
	content, err := valPackage.ToText()
	if err != nil {
		return nil, syscall.EIO
	}
	bytes := []byte(*content)

	end := off + int64(len(dest))
	if end > int64(len(bytes)) {
		end = int64(len(bytes))
	}
	return fuse.ReadResultData(bytes[off:end]), syscall.F_OK
}

func (c *ValFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (written uint32, errno syscall.Errno) {
	prevExtVal, err := c.GetExtendedData(ctx)
	common.Logger.Info("Writing to val file", "val", prevExtVal.GetId())

	newValPackage := NewValPackage(c.client, prevExtVal)
	if err != nil {
		common.Logger.Error("Error updating val package", "error", err)
		return 0, syscall.EIO
	}
	newValPackage.UpdateVal(string(data))
	extVal := newValPackage.Val

	err = c.parent.GetValOps().UpdateCode(ctx, prevExtVal.GetId(), newValPackage.Val.GetCode())
	if err != nil {
		common.Logger.Error("Error updating val code", "error", err)
		return 0, syscall.EIO
	}

	// Update metadata if needed
	if !c.client.Config.StaticMeta {
		extVal, err = c.parent.GetValOps().Read(ctx, prevExtVal.GetId())
		if err != nil {
			common.Logger.Error("Error fetching val", "error", err)
			return 0, syscall.EIO
		}
		c.ExtendedData = extVal
		c.ModifiedNow()
	} else {
		c.ModifiedAt = time.Now().Add(-1 * time.Second)
	}

	filename := ConstructFilename(c.BasicData.GetName(), ValType(c.BasicData.GetType()))
	waitThenMaybeDenoCache(filename, c.client)

	return uint32(len(data)), syscall.F_OK
}

func (f *ValFile) Getattr(
	ctx context.Context,
	fh fs.FileHandle,
	out *fuse.AttrOut,
) syscall.Errno {
	common.Logger.Info("Getting attributes for val file", "name", f.BasicData.Name)

	if f.ExtendedData == nil {
		out.Mode = VAL_FILE_FLAGS
		out.Size = uint64(len(f.BasicData.GetCode()) + 500)
		modified := time.Unix(0, 0)
		out.SetTimes(&modified, &modified, &modified)
		return 0
	}

	valPackage := NewValPackage(f.client, f.ExtendedData)
	contentLen, err := valPackage.Len()
	if err != nil {
		common.Logger.Error("Error getting content length", "error", err)
		return syscall.EIO
	}

	out.Size = uint64(contentLen)
	out.Mode = VAL_FILE_FLAGS

	modified := &f.ModifiedAt
	out.SetTimes(modified, modified, modified)

	common.Logger.Info("Got attributes for val file",
		"name", f.BasicData.Name,
		"size", out.Size,
		"mode", out.Mode,
		"modified", *modified)

	return syscall.F_OK
}

func (f *ValFile) Setattr(
	ctx context.Context,
	fh fs.FileHandle,
	in *fuse.SetAttrIn,
	out *fuse.AttrOut,
) syscall.Errno {
	common.Logger.Info("Setting attributes for val file", "name", f.BasicData.Name)

	out.Size = in.Size
	out.Mode = VAL_FILE_FLAGS
	out.Atime = in.Atime
	out.Mtime = in.Mtime
	out.Ctime = in.Ctime

	return syscall.F_OK
}
