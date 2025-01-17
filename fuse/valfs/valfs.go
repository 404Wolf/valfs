package fuse

import (
	"context"
	"log"
	"os/exec"
	"syscall"
	"time"

	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	common "github.com/404wolf/valfs/common"
	deno "github.com/404wolf/valfs/fuse/valfs/deno"
	blobs "github.com/404wolf/valfs/fuse/valfs/myblobs"
	myvals "github.com/404wolf/valfs/fuse/valfs/myvals"
	valfile "github.com/404wolf/valfs/fuse/valfs/myvals/valfile"
)

// Top level inode of a val file system
type ValFS struct {
	fs.Inode
	MountDir string
	client   *common.Client
}

// Create a new ValFS top level inode
func NewValFS(mountDir string, client *common.Client) *ValFS {
	return &ValFS{
		MountDir: mountDir,
		client:   client,
	}
}

func (c *ValFS) AddMyValsDir(ctx context.Context) {
	myValsDir := myvals.NewMyVals(&c.Inode, c.client, ctx)
	c.AddChild("myvals", &myValsDir.Inode, true)
}

func (c *ValFS) AddBlobsDir(ctx context.Context) {
	myValsDir := blobs.NewBlobs(&c.Inode, c.client, ctx)
	c.AddChild("myblobs", &myValsDir.Inode, true)
}

// Add the deno.json file which provides the user context about how to run and
// edit their vals
func (c *ValFS) AddDenoJSON(ctx context.Context) {
	denoJsonInode := deno.NewDenoJson(&c.Inode, c.client, ctx)
	c.AddChild("deno.json", denoJsonInode, false)
}

// Mount the filesystem
func (c *ValFS) Mount() error {
	log.Printf("Mounting ValFS file system at %s", c.MountDir)
	server, err := fs.Mount(c.MountDir, c, &fs.Options{
		MountOptions: fuse.MountOptions{
			Debug: false,
		},
		OnAdd: func(ctx context.Context) {
			ticker := time.NewTicker(5 * time.Second)
			go func() {
				for range ticker.C {
					log.Printf("Caching Deno libraries")
					log.Print("Executing deno cache --allow-import " + c.MountDir + "/myvals")
					cmd := exec.Command("deno", "cache", "--allow-import", c.MountDir+"/myvals")
					cmd.Start()
					cmd.Process.Release()
				}
			}()

			c.AddMyValsDir(ctx) // Add the folder with all the vals
			c.AddBlobsDir(ctx)  // Add the folder with all the blobs
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
	valFile, err := valfile.NewValFileFromExtendedVal(val, c.client)
	if err != nil {
		log.Fatal("Error creating val file", err)
	}
	c.Inode.NewPersistentInode(
		ctx,
		valFile,
		fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})

	return valFile
}
