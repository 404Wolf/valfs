package valfs

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	common "github.com/404wolf/valfs/common"
	deno "github.com/404wolf/valfs/valfs/deno"
	myvals "github.com/404wolf/valfs/valfs/myvals"
	valfile "github.com/404wolf/valfs/valfs/myvals"
)

// Top level inode of a val file system
type ValFS struct {
	fs.Inode
	client           *common.Client
	denoCacheLastRun time.Time
	denoCacheMutex   sync.Mutex
}

// Create a new ValFS top level inode
func NewValFS(client *common.Client) *ValFS {
	return &ValFS{client: client}
}

func (c *ValFS) AddMyValsDir(ctx context.Context) {
	myValsDir := myvals.NewMyVals(&c.Inode, c.client, ctx)
	c.AddChild("myvals", &myValsDir.Inode, true)
}

// Add the deno.json file which provides the user context about how to run and
// edit their vals
func (c *ValFS) AddDenoJSON(ctx context.Context) {
	denoJsonInode := deno.NewDenoJson(&c.Inode, c.client, ctx)
	c.AddChild("deno.json", denoJsonInode, false)
}

// Mount the filesystem
func (c *ValFS) Mount(doneSettingUp func()) error {
	c.client.Logger.Info("Mounting ValFS file system at ", c.client.Config.MountPoint)
	server, err := fs.Mount(c.client.Config.MountPoint, c, &fs.Options{
		MountOptions: fuse.MountOptions{
			Debug: c.client.Config.GoFuseDebug,
		},

		OnAdd: func(ctx context.Context) {
			// Add the folder with all the vals
			if c.client.Config.EnableValsDirectory {
				c.AddMyValsDir(ctx)
			}

			// Add the deno.json file
			if c.client.Config.DenoJson {
				c.AddDenoJSON(ctx)
			}

			doneSettingUp() // Callback
		},
	})

	if err != nil {
		c.client.Logger.Fatal("Mount failed", "error", err)
		return err
	}
	if c.client.Config.AutoUnmountOnExit {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-signalChan
			c.client.Logger.Info("Received interrupt signal. Unmounting...")
			err := server.Unmount()
			if err != nil {
				c.client.Logger.Error("Error unmounting", "error", err)
			}
			os.Exit(1)
		}()

		defer func() {
			c.client.Logger.Info("Unmounting", "mountPoint", c.client.Config.MountPoint)
			err := server.Unmount()
			if err != nil {
				c.client.Logger.Error("Error unmounting", "error", err)
			}
		}()
		c.client.Logger.Info("Unmount by calling 'umount'", "mountPoint", c.client.Config.MountPoint)
	}

	server.Wait()
	return nil
}

func (c *ValFS) RunDenoCache(glob string) {
	c.denoCacheMutex.Lock()

	// Check if enough time has passed since last run
	if time.Since(c.denoCacheLastRun) < time.Second {
		return
	}

	// If the vals dir is enabled, execute a deno cache on it
	if c.client.Config.EnableValsDirectory {
		go func() {
			defer c.denoCacheMutex.Unlock()

			c.client.Logger.Info("Caching Deno libraries")
			cacheCmd := c.client.Config.MountPoint + "/myvals" + glob
			c.client.Logger.Info("Executing deno cache", "command", "deno cache --allow-import "+cacheCmd)
			cmd := exec.Command("deno", "cache", "--allow-import", cacheCmd)
			cmd.Start()
			cmd.Process.Release()
		}()
	}

	// Update the last cache time to allow for a cooldown
	c.denoCacheLastRun = time.Now()
}

// Create a new ValFile object and corresponding inode from a basic val instance
func (c *ValFS) ValToValFile(
	ctx context.Context,
	val valgo.ExtendedVal,
) *valfile.ValFile {
	valFile, err := valfile.NewValFileFromExtendedVal(val, c.client)
	if err != nil {
		c.client.Logger.Fatal("Error creating val file", "error", err)
	}
	c.Inode.NewPersistentInode(
		ctx,
		valFile,
		fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})

	return valFile
}
