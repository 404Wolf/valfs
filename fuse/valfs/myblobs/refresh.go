package fuse

import (
	"context"
	"log"
	"net/http"
	"sync"
	"syscall"

	common "github.com/404wolf/valfs/common"
	blobfile "github.com/404wolf/valfs/fuse/valfs/myblobs/blobfile"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/subosito/gozaru"
)

var previousBlobKeys = make(map[string]*blobfile.BlobFile)

// Refresh the list of blobs in the filesystem
func refreshBlobs(
	ctx context.Context,
	root *fs.Inode,
	blobNameToKey *sync.Map,
	client common.Client,
) error {
	newBlobs, err := getMyBlobs(ctx, client)
	if err != nil {
		common.ReportError("Error fetching blobs", err)
		return nil
	}

	log.Printf("Fetched %d blobs", len(newBlobs))

	// Update or add new blobs
	for _, newBlob := range newBlobs {
		if prevBlobFile, exists := previousBlobKeys[newBlob.Key]; exists {
			// Update existing blob if it's newer
			if newBlob.GetLastModified().After(prevBlobFile.BlobListing.GetLastModified()) {
				prevBlobFile.BlobListing = newBlob
				prevBlobFile.EmbeddedInode().Root().NotifyContent(0, 0)
				log.Printf("Updated blob %s, found newer on valtown", newBlob.Key)
			}
		} else {
			// Add new blob
			blobFile := blobfile.NewBlobFile(newBlob, &client)
			newInode := root.NewPersistentInode(ctx, blobFile, fs.StableAttr{Mode: syscall.S_IFREG})
			sanitizedFilename := gozaru.Sanitize(newBlob.Key)
			blobNameToKey.Store(sanitizedFilename, newBlob.Key)
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
			blobNameToKey.Delete(gozaru.Sanitize(key))
			log.Printf("Removed blob %s no longer found on valtown", key)
		}
	}

	return nil
}

// Get a list of all the blobs belonging to the authed user
func getMyBlobs(ctx context.Context, client common.Client) ([]valgo.BlobListingItem, error) {
	log.Println("Fetching all of my blobs")

	blobs, resp, err := client.APIClient.BlobsAPI.BlobsList(ctx).Execute()
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, err
	}

	return blobs, nil
}
