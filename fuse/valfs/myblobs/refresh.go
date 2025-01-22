package fuse

import (
	"context"
	"log"
	"os"
	"syscall"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/subosito/gozaru"
)

var previousBlobKeys = make(map[string]*BlobFile)

// Refresh the list of blobs in the filesystem
func refreshBlobs(
	ctx context.Context,
	root *fs.Inode,
	blobNameToKey *cmap.ConcurrentMap[string, string],
	client common.Client,
	myBlobs *MyBlobs,
) error {
	newBlobs, err := getMyBlobs(ctx, client)
	if err != nil {
		common.ReportError("Error fetching blobs", err)
		return nil
	}

	log.Printf("Fetched %d blobs", len(newBlobs))

	// Update or add new blobs
	for _, newBlob := range newBlobs {
		// Previous blob keys is a decent source, but it's not perfect. It requires
		// no IO, so we start with it.
		if prevBlobFile, exists := previousBlobKeys[newBlob.Key]; exists {
			// Now, we see if it exists in the file system (is being/was created)
			_, err := os.Stat(prevBlobFile.TempFilePath())
			if os.IsNotExist(err) {
				// Update existing blob if it's newer
				if newBlob.GetLastModified().After(prevBlobFile.BlobListing.GetLastModified()) {
					prevBlobFile.BlobListing = newBlob
					prevBlobFile.EmbeddedInode().Root().NotifyContent(0, 0)
					log.Printf("Updated blob %s, found newer on valtown", newBlob.Key)
				}
			}
		} else {
			// Add new blob
			blobFile := NewBlobFileAuto(newBlob, &client, myBlobs)
			newInode := root.NewPersistentInode(ctx, blobFile, fs.StableAttr{Mode: syscall.S_IFREG})
			sanitizedFilename := gozaru.Sanitize(newBlob.Key)
			blobNameToKey.Set(sanitizedFilename, newBlob.Key)
			root.AddChild(sanitizedFilename, newInode, true)
			previousBlobKeys[newBlob.Key] = blobFile
			log.Printf("Added blob %s, found fresh on valtown", newBlob.Key)
		}
	}

	// Remove blobs that no longer exist
	for key := range previousBlobKeys {
		found := false
		for _, newBlob := range newBlobs {
			if newBlob.Key == key {
				found = true
				break
			}
		}
		if !found {
			root.RmChild(key)
			delete(previousBlobKeys, key)
			blobNameToKey.Remove(gozaru.Sanitize(key))
			log.Printf("Removed blob %s no longer found on valtown", key)
		}
	}

	return nil
}

// Get a list of all the blobs belonging to the authed user
func getMyBlobs(ctx context.Context, client common.Client) ([]valgo.BlobListingItem, error) {
	log.Println("Fetching all of my blobs")

	blobs, _, err := client.APIClient.BlobsAPI.BlobsList(ctx).Execute()
	if err != nil {
		return nil, err
	}

	return blobs, nil
}
