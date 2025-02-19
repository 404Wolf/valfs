package valfs

import (
	"context"
	"log"
	"syscall"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
)

// refreshBlobs updates the filesystem with the latest blob information
// It fetches the current list of blobs, adds new ones, updates existing ones,
// and removes those that no longer exist
func refreshBlobs(
	ctx context.Context,
	root *fs.Inode,
	myBlobs *MyBlobs,
) error {
	// Fetch the latest list of blobs from the server
	newBlobs, err := getMyBlobs(ctx, myBlobs.client)
	if err != nil {
		common.Logger.Error("Error fetching blobs", err)
		return nil
	}

	log.Printf("Fetched %d blobs", len(newBlobs))

	// Create a map to track blobs that still exist
	stillExistingBlobs := make(map[string]bool)

	// Iterate through the newly fetched blobs
	for _, newBlob := range newBlobs {
		stillExistingBlobs[newBlob.Key] = true

		inode := myBlobs.GetChild(newBlob.GetKey())
		if inode != nil {
			blobFile := inode.Operations().(*BlobFile)
			// The blob already exists, check if it needs updating
			if !blobFile.Upload.Ongoing() && newBlob.GetLastModified().After(blobFile.Meta.GetLastModified()) {
				blobFile.Meta = newBlob
				root.GetChild(newBlob.Key).NotifyContent(0, 0)
				log.Printf("Updated blob %s, found newer on valtown", newBlob.Key)
			}
		} else {
			// This is a new blob, add it to the filesystem
			blobFile := NewBlobFileAuto(newBlob, myBlobs)
			newInode := root.NewPersistentInode(ctx, blobFile, fs.StableAttr{Mode: syscall.S_IFREG})
			root.AddChild(newBlob.Key, newInode, true) // TODO this is nullable
			log.Printf("Added blob %s, found fresh on valtown", newBlob.Key)
		}
	}

	// Check for blobs that no longer exist and remove them
	inodes := myBlobs.Children()
	for name := range inodes {
		if !stillExistingBlobs[name] {
			// This blob no longer exists, remove it from the filesystem and map
			root.RmChild(name)
			log.Printf("Removed blob %s no longer found on valtown", name)
		}
	}

	return nil
}

// getMyBlobs fetches the list of all blobs belonging to the authenticated user
// It uses the provided client to make an API call to retrieve the blob listings
func getMyBlobs(ctx context.Context, client *common.Client) ([]valgo.BlobListingItem, error) {
	log.Println("Fetching all of my blobs")

	// Make an API call to fetch the list of blobs
	blobs, _, err := client.APIClient.BlobsAPI.BlobsList(ctx).Execute()
	if err != nil {
		return nil, err
	}

	return blobs, nil
}
