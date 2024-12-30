package fuse

import (
	"github.com/hanwen/go-fuse/v2/fuse"
)

type MyServerCallbacks struct {
	Conn *fuse.RawFileSystem
}

func (m *MyServerCallbacks) InodeNotify(node uint64, off int64, length int64) fuse.Status {
	return fuse.OK
}
