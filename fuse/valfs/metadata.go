package fuse

import (
	"context"

	_ "embed"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

//go:embed deno.json
var denoJSON []byte

// Add the deno.json file which provides the user context about how to run and
// edit their vals
func (c *ValFS) AddDenoJSON(ctx context.Context) {
	ch := c.NewPersistentInode(
		ctx, &fs.MemRegularFile{
			Data: denoJSON,
			Attr: fuse.Attr{
				Mode: 0644,
			},
		}, fs.StableAttr{Ino: 2})

	c.AddChild("deno.json", ch, false)
}
