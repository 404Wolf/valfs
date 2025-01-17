package fuse

import (
	"context"
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
	resp, err := c.client.APIClient.BlobsAPI.BlobsStore(ctx, name).
		Body(tempFile).
		Execute()
	if err != nil {
		common.ReportError("Error creating blob", err)
		return nil, nil, 0, syscall.EIO
	} else if resp.StatusCode != http.StatusCreated {
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
	flags uint32,
) syscall.Errno {
	if newParent.EmbeddedInode().StableAttr().Ino != c.Inode.StableAttr().Ino {
		log.Printf("Cannot move blob out of the `myblobs` directory")
		return syscall.EINVAL
	}

	oldKey, _ := getKeyFromBlobName(oldName)

	if c.GetChild(newName) != nil {
		return syscall.EEXIST
	}

	inode := c.GetChild(*oldKey)
	if inode == nil {
		return syscall.ENOENT
	}
	blobFile := inode.Operations().(*blobfile.BlobFile)

	resp, err := c.client.APIClient.BlobsAPI.BlobsStore(ctx, newName).Body(blobFile.File).Execute()
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Error storing blob with new key. Err: %v, Resp: %v", err, resp)
		return syscall.EIO
	}

	resp, err = c.client.APIClient.BlobsAPI.BlobsDelete(ctx, *oldKey).Execute()
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Error deleting old blob. Err: %v, Resp: %v", err, resp)
		c.rollbackRename(ctx, newName)
		return syscall.EIO
	}

	updatedBlob, resp, err := c.client.APIClient.BlobsAPI.BlobsGet(ctx, newName).Execute()
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Error fetching updated blob. Err: %v, Resp: %v", err, resp)
		return syscall.EIO
	}
	blobFile.File = updatedBlob
	blobFile.BlobListing.Key = newName

	return syscall.F_OK
}

// Attempts to delete the newly created blob in case of a failure
func (c *MyBlobs) rollbackRename(ctx context.Context, newKey string) {
	resp, err := c.client.APIClient.BlobsAPI.BlobsDelete(ctx, newKey).Execute()
	if err != nil {
		log.Printf("Error rolling back rename (deleting new blob). Err: %v", err)
	} else if resp.StatusCode != http.StatusOK {
		log.Printf("Error rolling back rename (deleting new blob). Resp: %v", resp)
	}
}
