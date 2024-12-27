package fuse

import (
	"context"
	"log"
	"syscall"

	utils "github.com/404wolf/valfs/fuse/utils"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Get the code of a val padded and ready for placement in a file
func getContents(val *valgo.BasicVal) string {
	return utils.AffixShebang(val.GetCode())
}

var _ = (fs.NodeOpener)((*ValFile)(nil))

// Provide the content of the val as the content of the file
func (fh *BytesFileHandle) Read(
	ctx context.Context,
	dest []byte,
	off int64,
) (fuse.ReadResult, syscall.Errno) {
	end := off + int64(len(dest))
	if end > int64(len(fh.content)) {
		end = int64(len(fh.content))
	}
	log.Printf("Reading from %d to %d", off, end)

	return fuse.ReadResultData(fh.content[off:end]), 0
}

const DefaultValContents = `console.log("Hello world!")`
const DefaultValType = Script

// Handle requests for a file descriptor
func (f *ValFile) Open(ctx context.Context, openFlags uint32) (
	fh fs.FileHandle,
	fuseFlags uint32,
	errno syscall.Errno,
) {
	// Provide the Val's code as the data
	contents := getContents(&f.ValData)
	fh = &BytesFileHandle{
		content: []byte(contents),
	}

	// Return FOPEN_DIRECT_IO so content is not cached
	return fh, fuse.FOPEN_DIRECT_IO, 0
}
