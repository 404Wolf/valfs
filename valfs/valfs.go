package valfs

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	common "github.com/404wolf/valfs/common"
	editor "github.com/404wolf/valfs/valfs/editor"
	vals "github.com/404wolf/valfs/valfs/vals"
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

// Add a directory that is the root for all your vals. All vals will be loosly
// placed in this folder.
func (c *ValFS) AddValsDir(ctx context.Context) {
	common.Logger.Info("Adding vals directory to valfs")
	valsDir := vals.NewRegularValsDir(&c.Inode, c.client, ctx)
	c.AddChild("vals", valsDir.GetInode(), true)
}

// Add a directory that is the root for all your val town projects. All val town projects will be
// placed in this folder, where each one gets a folder
func (c *ValFS) AddProjectsDir(ctx context.Context) {
	common.Logger.Info("Adding vals directory to valfs")
	projectsDir := vals.NewProjectsDir(&c.Inode, c.client, ctx)
	c.AddChild("projects", projectsDir.GetInode(), true)
}

// Add the deno.json file which provides the user context about how to run and
// edit their vals
func (c *ValFS) AddDenoJSON(ctx context.Context) {
	common.Logger.Info("Adding deno.json to valfs")
	denoJsonInode := editor.NewDenoJson(&c.Inode, c.client, ctx)
	c.AddChild("deno.json", denoJsonInode, false)
}

// Mount the filesystem
func (c *ValFS) Mount(doneSettingUp func()) error {
	common.Logger.Info("Mounting ValFS file system at ", c.client.Config.MountPoint)

	server, err := fs.Mount(c.client.Config.MountPoint, c, &fs.Options{
		MountOptions: fuse.MountOptions{
			Debug: c.client.Config.GoFuseDebug,
		},

		OnAdd: func(ctx context.Context) {
			// Add the folder with all the vals
			if c.client.Config.EnableValsDirectory {
				c.AddValsDir(ctx)
			}

			// Add the folder with all the projects
			if c.client.Config.EnableProjectsDirectory {
				c.AddProjectsDir(ctx)
			}

			// Add the deno.json file
			if c.client.Config.DenoJson {
				c.AddDenoJSON(ctx)
			}

			doneSettingUp() // Callback
		},
	})

	if err != nil {
		common.Logger.Fatal("Mount failed", "error", err)
		return err
	}
	if c.client.Config.AutoUnmountOnExit {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-signalChan
			common.Logger.Info("Received interrupt signal. Unmounting...")
			err := server.Unmount()
			if err != nil {
				common.Logger.Error("Error unmounting", "error", err)
			}
			os.Exit(1)
		}()

		defer func() {
			common.Logger.Info("Unmounting valfs @ %s", c.client.Config.MountPoint)
			err := server.Unmount()
			if err != nil {
				common.Logger.Error("Error unmounting", "error", err)
			}
		}()
		common.Logger.Info("Unmount by calling 'umount' ", c.client.Config.MountPoint)
	}

	server.Wait()
	return nil
}
