package fuse

import (
	"context"
	"errors"
	"io"
	"log"
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
	"github.com/subosito/gozaru"

	common "github.com/404wolf/valfs/common"
)

const BLOB_REFRESH_INTERVAL = 5

var blobUuidToKey sync.Map
var blobNameToKey sync.Map

var validFilenameRegex = regexp.MustCompile(`^[^\x00-\x1f/]+$`)

func validateName(filename string) bool {
	return validFilenameRegex.MatchString(filename) && filename != "." && filename != ".."
}

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
func NewBlobs(parent *fs.Inode, client *common.Client, ctx context.Context) *MyBlobs {
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
	key, ok := getKeyFromBlobName(name)
	if !ok {
		return syscall.ENOENT
	}
	log.Printf("Deleting blob %s", *key)

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

	log.Printf("Creating blob %s", name)

	content := []byte("{}")
	tempFile, err := os.CreateTemp("", "valfs-blob-*")
	if err != nil {
		common.ReportError("Failed to create temp file", err)
		return nil, nil, 0, syscall.EIO
	}
	_, err = tempFile.Write(content)
	tempFile.Seek(0, 0)

	// For reading blobs, we use a raw http request so that we can stream the
	// data to the file. For creating a new blob, we can just use the SDK's
	// function, which accepts an *os.File. We don't do this for reading because
	// we don't want to load large amounts of data into memory (it seems like it
	// does in its source, but will need to be investigated further), but here we
	// are only loading "{}" into memory.
	_, err = c.client.APIClient.BlobsAPI.BlobsStore(ctx, name).
		Body(tempFile).
		Execute()
	if err != nil {
		common.ReportError("Error creating blob", err)
		return nil, nil, 0, syscall.EIO
	}
	log.Printf("Created blob %v successfully", name)

	blobListingItem := valgo.NewBlobListingItem(name)
	blobListingItem.SetSize(int32(len(content)))
	blobFile := blobfile.NewBlobFile(*blobListingItem, c.client)
	newInode := c.NewInode(
		ctx,
		blobFile,
		fs.StableAttr{Mode: syscall.S_IFREG, Ino: blobFile.UUID})
	fileHandle := &blobfile.BlobFileHandle{File: tempFile}

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
	if newParent.EmbeddedInode().StableAttr().Ino != c.Inode.StableAttr().Ino {
		log.Printf("Cannot move blob out of the `myblobs` directory")
		return syscall.EINVAL
	}

	// Update metadata for the blob (do book keeping)
	oldKey, _ := getKeyFromBlobName(oldName)
	if oldKey == nil {
		common.ReportError("Blob not found", errors.New("Blob not found"))
		return syscall.ENOENT
	}

	newKey := gozaru.Sanitize(newName)
	inode := c.GetChild(*oldKey)
	if inode == nil {
		return syscall.ENOENT
	}
	blobFile := inode.Operations().(*blobfile.BlobFile)

	// Start a transaction to do the rename
	err := c.renameTransaction(ctx, oldKey, newKey)
	if err != nil {
		common.ReportError("Error renaming blob", err)
		return syscall.EIO
	}

	// Update the local metadata
	blobFile.BlobListing.Key = newKey

	return syscall.F_OK
}

func (c *MyBlobs) renameTransaction(ctx context.Context, oldKey *string, newKey string) error {
	// Fetch the old blob
	getResp, err := c.client.APIClient.RawRequest(http.MethodGet, "/v1/blob/"+*oldKey, nil)
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
	storeResp, err := c.client.APIClient.RawRequest(http.MethodPost, "/v1/blob/"+newKey, pr)
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
	deleteResp, err := c.client.APIClient.RawRequest("DELETE", "/v1/blob/"+newKey, nil)
	if err != nil {
		common.ReportError("Error rolling back rename", err)
		return
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		common.ReportErrorResp("Error rolling back rename", deleteResp)
	}
}
