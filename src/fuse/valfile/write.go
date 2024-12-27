package fuse

import (
	"context"
	"syscall"

	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
)

var _ = (fs.NodeWriter)((*ValFile)(nil))

func (c *ValFile) Write(
	ctx context.Context,
	fh fs.FileHandle,
	data []byte,
	off int64,
) (written uint32, errno syscall.Errno) {
	oldData := (fh.(*BytesFileHandle)).content
	newData := append(oldData[:off], data...)

	valData := &c.ValData
	valData.SetCode(string(newData))

	valsCreateReq := valgo.NewValsCreateRequest(valData.GetCode())
	valsCreateReq.SetName(valData.GetName())
	valsCreateReq.SetType(valData.GetType())
	extVal, resp, err := c.ValClient.ValsAPI.ValsCreateVersion(ctx, valData.GetId()).ValsCreateRequest(*valsCreateReq).Execute()
	if err != nil || resp.StatusCode != 200 {
		return 0, syscall.EIO
	}
	c.ValData = extVal.ToBasicVal()

	return uint32(len(data)), 0
}
