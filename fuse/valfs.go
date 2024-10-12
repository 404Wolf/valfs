package fuse

import (
	"context"
	"github.com/404wolf/valfs/sdk"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"log"
	"syscall"
	"time"
)

// A container for a val file system, with metadata about the file system
type ValFS struct {
	ValClient *sdk.ValTownClient
	MountDir  string
}

// Create a new val file system instance
func NewValFS(valTownClient *sdk.ValTownClient, mountDir string) *ValFS {
	return &ValFS{
		ValClient: valTownClient,
		MountDir:  mountDir,
	}
}

// Mount the filesystem
func (c *ValFS) Mount() error {
	root := &fs.Inode{}
	log.Println("Mounted root Inode")

	server, err := fs.Mount(c.MountDir, root, &fs.Options{
		MountOptions: fuse.MountOptions{Debug: false},
		OnAdd: func(ctx context.Context) {
			c.refreshVals(ctx, root) // Initial load
			ticker := time.NewTicker(5 * time.Second)
			go func() {
				for range ticker.C {
					c.refreshVals(ctx, root)
				}
			}()
		},
	})

	if err != nil {
		log.Fatal(err)
		return err
	}

	log.Printf("Unmount by calling 'fusermount -u %s'\n", c.MountDir)
	server.Wait()
	return nil
}

// Refresh the list of vals in the filesystem
func (c *ValFS) refreshVals(ctx context.Context, root *fs.Inode) {
	resp, err := c.ValClient.Vals.OfMine()
	if err != nil {
		log.Fatal(err)
	}

	for _, val := range resp {
		log.Printf("Adding val %s", val.Name)
		valFile := &valFile{
			ValData: val,
		}

		ch := root.NewPersistentInode(
			ctx,
			valFile,
			fs.StableAttr{Mode: syscall.S_IFREG}) // regular file
		root.AddChild(val.Name+".tsx", ch, true)
		log.Printf("Added child %s", val.Name)
	}
}
