package fuse

import (
	"context"
	"fmt"
	"github.com/404wolf/valfs/sdk"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"log"
	"syscall"
)

// A file handle that carries separate content seperately for each Open call
type bytesFileHandle struct {
	content []byte
}

// A file that contains the code of a val
type valFile struct {
	fs.Inode
	ValData   sdk.ValData
	ValClient *sdk.ValTownClient
}

// Provide the content of the val as the content of the file
func (fh *bytesFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	end := off + int64(len(dest))
	if end > int64(len(fh.content)) {
		end = int64(len(fh.content))
	}
	log.Printf("Reading from %d to %d", off, end)

	return fuse.ReadResultData(fh.content[off:end]), 0
}

// Handle requests for a file descriptor. Tell the user that it is an executable read/write file.
func (f *valFile) Open(ctx context.Context, openFlags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	// Provide the Val's code as the data
	contents := executeValShebang(f.ValData.Code)
	fh = &bytesFileHandle{
		content: []byte(contents),
	}

	// Return FOPEN_DIRECT_IO so content is not cached.
	return fh, fuse.FOPEN_DIRECT_IO, 0
}

// Handle deletion of a file. For now, do nothing.
func (f *valFile) Unlink(ctx context.Context, name string) syscall.Errno {
	fmt.Println("Unlinking", name)
	return 0
}

// Handler for getting metadata for a file; says it is read/write/executable
func (f *valFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}
