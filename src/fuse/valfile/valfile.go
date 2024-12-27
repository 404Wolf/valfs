package fuse

import (
	"syscall"

	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
)

const ValFileMode = syscall.S_IFREG | 0o777

var _ = (fs.FileReader)((*BytesFileHandle)(nil))

// A file in the val file system, with metadata about the file and an inode
type ValFile struct {
	fs.Inode
	ValData   valgo.BasicVal
	ValClient *valgo.APIClient
}

// A file handle that carries separate content for each open call
type BytesFileHandle struct {
	content []byte
}
