package fuse

import (
	"context"
	"syscall"

	_ "embed"

	common "github.com/404wolf/valfs/common"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type DenoJson struct {
	fs.Inode
	client *common.Client
}

//go:embed deno.json
var denoJSON []byte

func NewDenoJson(parent *fs.Inode, client *common.Client, ctx context.Context) *fs.Inode {
	denoJsonFile := &fs.MemRegularFile{
		Data: denoJSON,
		Attr: fuse.Attr{
			Mode: 0644,
		},
	}

	return parent.NewPersistentInode(
		ctx,
		denoJsonFile,
		fs.StableAttr{Ino: 2, Mode: syscall.S_IFREG},
	)
}
