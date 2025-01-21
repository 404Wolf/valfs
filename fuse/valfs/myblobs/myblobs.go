package fuse

import (
	"context"
	"errors"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"regexp"
	"sync"
	"syscall"
	"time"

	blobfile "github.com/404wolf/valfs/fuse/valfs/myblobs/blobfile"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	common "github.com/404wolf/valfs/common"
)

const BLOB_REFRESH_INTERVAL = 5

var blobUuidToKey sync.Map
var blobNameToKey sync.Map

var validFilenameRegex = regexp.MustCompile(`^[^\x00-\x1f/]+$`)

// validate a filename to make sure it can be used as a blobstore key. We could
// sanitize instead but it's easier to not do an additional rename after the
// user does a rename.
func validateName(filename string) bool {
	return validFilenameRegex.MatchString(filename) && filename != "." && filename != ".."
}

// getKeyFromBlobName gets the key of the blob from the blob name
func getKeyFromBlobName(name string) (*string, bool) {
	key, ok := blobNameToKey.Load(name)
	if !ok {
		return nil, false
	}
	keyStr := key.(string)
	return &keyStr, true
}

// The folder with all of my blobs in it
type MyBlobs struct {
	fs.Inode
	client *common.Client
}

// Set up background refresh of blobs and retrieve an auto updating folder of
// blob files
func NewMyBlobs(parent *fs.Inode, client *common.Client, ctx context.Context) *MyBlobs {
	myBlobsDir := &MyBlobs{
		client: client,
	}
	attrs := fs.StableAttr{Mode: syscall.S_IFDIR | 0555}
	parent.NewInode(ctx, myBlobsDir, attrs)

	refreshBlobs(ctx, &myBlobsDir.Inode, &blobNameToKey, *client)
	ticker := time.NewTicker(BLOB_REFRESH_INTERVAL * time.Second)
	go func() {
		for range ticker.C {
			refreshBlobs(ctx, &myBlobsDir.Inode, &blobNameToKey, *client)
			log.Println("Refreshed blobs")
		}
	}()

	return myBlobsDir
}

var _ = (fs.NodeUnlinker)((*MyBlobs)(nil))

// Handle deletion of a file by also deleting the blob
func (c *MyBlobs) Unlink(ctx context.Context, name string) syscall.Errno {
	// Get the key of the blob
	key, ok := getKeyFromBlobName(name)
	if !ok {
		return syscall.ENOENT
	}
	log.Printf("Deleting blob %s", *key)

	// Attempt to delete the blob
	_, err := c.client.APIClient.BlobsAPI.BlobsDelete(ctx, *key).Execute()
	if err != nil {
		log.Printf("Error deleting blob %s: %v", *key, err)
		return syscall.EIO
	} else {
		log.Printf("Deleted blob %s", *key)
	}

	// Remove the blobFile from the map
	uuid, ok := blobNameToKey.Load(name)
	if !ok {
		return syscall.EIO
	}
	blobUuidToKey.Delete(uuid.(string))

	return syscall.F_OK
}

var _ = (fs.NodeCreater)((*MyBlobs)(nil))

// Create a new blob on new file creation
func (c *MyBlobs) Create(
	ctx context.Context,
	name string,
	flags uint32,
	mode uint32,
	out *fuse.EntryOut,
) (node *fs.Inode, fh fs.FileHandle, fuseFlags uint32, code syscall.Errno) {
	if ok := validateName(name); !ok {
		return nil, nil, 0, syscall.EINVAL
	}
	blobNameToKey.Store(name, name)

	log.Printf("Creating blob %s", name)

	// Determine the uuid for the new val file so we can create a temp file for
	// it ahead of time
	uuid := rand.Uint64()

	// The val town API does not give us the blob listing after we make it so we
	// guess what it would be instead of doing a second round trip
	blobListingItem := valgo.NewBlobListingItem(name)
	blobListingItem.SetKey(name)
	blobListingItem.SetSize(0)
	blobListingItem.SetLastModified(time.Now().Add(-10 * time.Second))
	blobFile := blobfile.NewBlobFile(*blobListingItem, c.client, uuid)

	// Create the empty blob on valtown
	tempFilePath := blobFile.TempFilePath()
	file, err := os.OpenFile(tempFilePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		common.ReportError("Failed to open temporary file", err)
		return nil, nil, 0, syscall.EIO
	}

	// Create the new inode
	newInode := c.NewPersistentInode(
		ctx,
		blobFile,
		fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})

	// Create the new entry in val town blob store
	resp, err := c.client.APIClient.RawRequest(
		ctx,
		http.MethodPost,
		"/v1/blob/"+name,
		file,
	)
	if err != nil {
		common.ReportError("Failed to create blob", err)
		return nil, nil, 0, syscall.EIO
	} else if resp.StatusCode != http.StatusCreated {
		common.ReportErrorResp("Failed to create blob", resp)
		return nil, nil, 0, syscall.EIO
	}

	// Open the file handle
	fileHandle, _, _ := blobFile.Open(ctx, flags)

	return newInode, fileHandle, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

var _ = (fs.NodeRenamer)((*MyBlobs)(nil))

func (c *MyBlobs) Rename(
	ctx context.Context,
	oldName string,
	newParent fs.InodeEmbedder,
	newName string,
	code uint32,
) syscall.Errno {
	// Prevent from moving it out of the directory
	if newParent.EmbeddedInode().StableAttr().Ino != c.Inode.StableAttr().Ino {
		log.Printf("Cannot move blob out of the `myblobs` directory")
		return syscall.EINVAL
	}

	// Make sure the new name is valid
	if ok := validateName(newName); !ok {
		return syscall.EINVAL
	}

	// Update metadata for the blob (do book keeping)
	oldKey, ok := getKeyFromBlobName(oldName)
	if !ok {
		common.ReportError("Blob not found", errors.New("Blob not found"))
		return syscall.ENOENT
	}

	inode := c.GetChild(*oldKey)
	if inode == nil {
		return syscall.ENOENT
	}
	blobFile := inode.Operations().(*blobfile.BlobFile)

	// Start a transaction to do the rename
	err := c.renameTransaction(ctx, oldKey, newName)
	if err != nil {
		common.ReportError("Error renaming blob", err)
		return syscall.EIO
	}

	// Update the local metadata
	blobFile.BlobListing.Key = newName

	// Update the name-to-key mapping
	blobNameToKey.Delete(oldName)
	blobNameToKey.Store(newName, newName)

	return syscall.F_OK
}

func (c *MyBlobs) renameTransaction(ctx context.Context, oldKey *string, newKey string) error {
	// Fetch the old blob
	getResp, err := c.client.APIClient.RawRequest(
		ctx,
		http.MethodGet,
		"/v1/blob/"+*oldKey,
		nil,
	)
	if err != nil {
		common.ReportError("Failed to fetch old blob data", err)
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
			common.ReportError("Error copying blob data", err)
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
		common.ReportError("Error storing blob with new key", err)
		return err
	}

	// Wait to finish writing before ending streaming
	wg.Wait()
	storeResp.Body.Close()

	// Check to see if the store was successful
	if storeResp.StatusCode != http.StatusCreated {
		common.ReportErrorResp("Error storing blob with new key", storeResp)
		return errors.New(common.ReportErrorResp("Failed to store new blob: %d", storeResp))
	}

	// If we've made it this far, the new blob is stored successfully.
	// Now we can safely delete the old blob.
	resp, err := c.client.APIClient.BlobsAPI.BlobsDelete(ctx, *oldKey).Execute()
	if err != nil {
		log.Printf("Error deleting blob %s: %v", *oldKey, err)
		return syscall.EIO
	} else if resp.StatusCode != http.StatusNoContent {
		return errors.New(common.ReportErrorResp("Error deleting blob", resp))
	} else {
		log.Printf("Deleted blob %s", *oldKey)
	}

	return nil
}

func (c *MyBlobs) rollbackRename(ctx context.Context, newKey string) {
	deleteResp, err := c.client.APIClient.RawRequest(
		ctx,
		http.MethodDelete,
		"/v1/blob/"+newKey,
		nil,
	)
	if err != nil {
		common.ReportError("Error rolling back rename", err)
		return
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		common.ReportErrorResp("Error rolling back rename", deleteResp)
	}
}
