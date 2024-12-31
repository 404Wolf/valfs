package fuse

import (
	"context"
	"log"
	"syscall"

	_ "embed"

	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	client "github.com/404wolf/valfs/client"
	valfile "github.com/404wolf/valfs/fuse/valfile"
	myvals "github.com/404wolf/valfs/fuse/valfs/myvals"
)

// Top level inode of a val file system
type ValFS struct {
	fs.Inode
	MountDir string
	client   *client.Client
}

// Create a new ValFS top level inode
func NewValFS(mountDir string, client *client.Client) *ValFS {
	return &ValFS{
		MountDir: mountDir,
		client:   client,
	}
}

func (c *ValFS) AddMyValsDir(ctx context.Context) {
	myValsDir := myvals.NewMyVals(&c.Inode, c.client, ctx)
	c.AddChild("myvals", &myValsDir.Inode, true)
}

// Mount the filesystem
func (c *ValFS) Mount() error {
	log.Printf("Mounting ValFS file system at %s", c.MountDir)
	server, err := fs.Mount(c.MountDir, c, &fs.Options{
		MountOptions: fuse.MountOptions{
			Debug: false,
		},
		OnAdd: func(ctx context.Context) {
			c.AddMyValsDir(ctx) // Add the folder with all the vals
			c.AddDenoJSON(ctx)  // Add the deno.json file
		},
	})

	if err != nil {
		log.Fatal(err)
		return err
	}

	log.Printf("Unmount by calling 'umount %s'\n", c.MountDir)
	server.Wait()
	return nil
}

// Create a new ValFile object and corresponding inode from a basic val instance
func (c *ValFS) ValToValFile(
	ctx context.Context,
	val valgo.ExtendedVal,
) *valfile.ValFile {
	valFile, err := valfile.NewValFile(val, c.client)
	if err != nil {
		log.Fatal("Error creating val file", err)
	}
	c.Inode.NewPersistentInode(
		ctx,
		valFile,
		fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})

	return valFile
}
