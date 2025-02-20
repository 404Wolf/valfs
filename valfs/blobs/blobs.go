package valfs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
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

var _ = (fs.NodeUnlinker)((*BlobsDir)(nil))
var _ = (fs.NodeRenamer)((*BlobsDir)(nil))

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

// Handle deletion of a file by also deleting the blob
func (c *BlobsDir) Unlink(ctx context.Context, name string) syscall.Errno {
	common.Logger.Info("Deleting blob " + name)

	// Attempt to delete the blob
	_, err := c.client.APIClient.BlobsAPI.BlobsDelete(ctx, name).Execute()
	if err != nil {
		common.Logger.Error("Error deleting blob "+name+": ", err)
		return syscall.EIO
	} else {
		common.Logger.Info("Deleted blob " + name)
	}

	return syscall.F_OK
}

// Rename the blob file. This simultaniously pulls the val from the valtown
// blob api, and then pipes the output over to a differnet TCP socket where we
// directly upload it to the api, but under a different key. If this succeeds
// we then delete the old version.
func (c *BlobsDir) Rename(
	ctx context.Context,
	oldName string,
	newParent fs.InodeEmbedder,
	newName string,
	code uint32,
) syscall.Errno {
	// Prevent from moving it out of the directory
	if newParent.EmbeddedInode().StableAttr().Ino != c.Inode.StableAttr().Ino {
		common.Logger.Info("Cannot move blob out of the `blobs` directory")
		return syscall.EINVAL
	}

	// Make sure the new name is valid
	if ok := validateName(newName); !ok {
		return syscall.EINVAL
	}

	// Start a transaction to do the rename
	err := c.renameTransaction(ctx, oldName, newName)
	if err != nil {
		common.Logger.Error("Error renaming blob", err)
		return syscall.EIO
	}

	return syscall.F_OK
}

func (c *BlobsDir) renameTransaction(ctx context.Context, oldKey string, newKey string) error {
	// Fetch the old blob
	getResp, err := c.client.APIClient.RawRequest(
		ctx,
		http.MethodGet,
		"/v1/blob/"+oldKey,
		nil,
	)
	if err != nil {
		common.Logger.Error("Failed to fetch old blob data", err)
		return err
	}
	defer getResp.Body.Close()

	// Prepare to store the new blob
	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer pw.Close()
		defer wg.Done()
		_, err := io.Copy(pw, getResp.Body)
		if err != nil {
			common.Logger.Error("Error copying blob data", err)
		}
	}()

	// Store the new blob
	storeResp, err := c.client.APIClient.RawRequest(
		ctx,
		http.MethodPost,
		"/v1/blob/"+newKey,
		pr,
	)
	if err != nil {
		common.Logger.Error("Error storing blob with new key", err)
		return err
	}

	// Wait to finish writing before ending streaming
	wg.Wait()
	storeResp.Body.Close()

	// Check to see if the store was successful
	if storeResp.StatusCode != http.StatusCreated {
		common.Logger.Error("Error storing blob with new key", storeResp)
		return fmt.Errorf("Failed to store new blob: %v", storeResp)
	}

	// If we've made it this far, the new blob is stored successfully.
	// Now we can safely delete the old blob.
	resp, err := c.client.APIClient.BlobsAPI.BlobsDelete(ctx, oldKey).Execute()
	if err != nil {
		common.Logger.Error("Error deleting blob "+oldKey+": ", err)
		return syscall.EIO
	} else if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Error deleting blob %v", resp)
	} else {
		common.Logger.Info("Deleted blob " + oldKey)
	}

	return nil
}

func (c *BlobsDir) rollbackRename(ctx context.Context, newKey string) {
	deleteResp, err := c.client.APIClient.RawRequest(
		ctx,
		http.MethodDelete,
		"/v1/blob/"+newKey,
		nil,
	)
	if err != nil {
		common.Logger.Error("Error rolling back rename", err)
		return
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		common.Logger.Error("Error rolling back rename", deleteResp)
	}
}
