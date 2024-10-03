package fuse

import (
	"context"
	"log"
	"syscall"

	"github.com/404wolf/valfs/sdk"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// bytesFileHandle is a file handle that carries separate content for
// each Open call
type bytesFileHandle struct {
	content []byte
}

// bytesFileHandle allows reads
var _ = (fs.FileReader)((*bytesFileHandle)(nil))

func (fh *bytesFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	end := off + int64(len(dest))
	if end > int64(len(fh.content)) {
		end = int64(len(fh.content))
	}

	return fuse.ReadResultData(fh.content[off:end]), 0
}

// A Val town Val as a File
type valInode struct {
	fs.Inode
}

type valFile struct {
	Inode   *valInode
	ValData *sdk.ValData
}

type ValFS struct {
	ValClient *sdk.ValTownClient
	ValAuthor *sdk.ValAuthor
	MountDir  string
}

func NewValFS(valClient *sdk.ValTownClient, valAuthor *sdk.ValAuthor, mountDir string) *ValFS {
	return &ValFS{
		ValClient: valClient,
		ValAuthor: valAuthor,
		MountDir:  mountDir,
	}
}

// Mount mounts the ValFS
func (c *ValFS) Mount() error {
	root := &fs.Inode{}
	log.Println("Mounted root Inode")

	server, err := fs.Mount(c.MountDir, root, &fs.Options{
		MountOptions: fuse.MountOptions{Debug: false},

		OnAdd: func(ctx context.Context) {
			username := string(c.ValAuthor.Username)
			resp, err := c.ValClient.Vals.Search(username)
			if err != nil {
				log.Fatal(err)
				panic(err)
			}

			for _, val := range resp.Data {
				log.Printf("Iterating through val %s", val.Name)
				ch := root.NewPersistentInode(
					ctx,
					&valInode{},
					fs.StableAttr{Mode: syscall.S_IFREG})
				root.AddChild(val.Name, ch, true)
				log.Printf("Added child %s", val.Name)
			}
		},
	})

	if err != nil {
		log.Fatal(err)
		return err
	}

	log.Printf("Unmount by calling 'fusermount -u %s'\n", c.MountDir)

	// Serve the file system until unmounted w/`fusermount -u`
	server.Wait()

	return nil
}
