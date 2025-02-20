package valfs

import (
	"context"
	"regexp"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"

	common "github.com/404wolf/valfs/common"
)

const BlobRequestTimeout = 5

var validFilenameRegex = regexp.MustCompile(`^[^\x00-\x1f/]+$`)

// validate a filename to make sure it can be used as a blobstore key. We could
// sanitize instead but it's easier to not do an additional rename after the
// user does a rename.
func validateName(filename string) bool {
	return validFilenameRegex.MatchString(filename) && filename != "." && filename != ".."
}

// The folder with all of my blobs in it
type BlobsDir struct {
	fs.Inode
	client *common.Client
}

// Set up background refresh of blobs and retrieve an auto updating folder of
// blob files
func NewBlobsDir(parent *fs.Inode, client *common.Client, ctx context.Context) *BlobsDir {
	blobsDir := &BlobsDir{client: client}
	attrs := fs.StableAttr{Mode: syscall.S_IFDIR | 0555}
	parent.NewInode(ctx, blobsDir, attrs)

	if client.Config.AutoRefresh {
		refreshBlobs(ctx, &blobsDir.Inode, blobsDir)
		ticker := time.NewTicker(time.Duration(client.Config.AutoRefreshInterval) * time.Second)
		go func() {
			for range ticker.C {
				refreshBlobs(ctx, &blobsDir.Inode, blobsDir)
				common.Logger.Info("Refreshed blobs")
			}
		}()
	}

	return blobsDir
}

