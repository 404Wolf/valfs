package fuse

import (
	"context"
	"log"
	"net/http"
	"syscall"

	client "github.com/404wolf/valfs/client"
	valfile "github.com/404wolf/valfs/fuse/valfile"
	"github.com/404wolf/valgo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/hanwen/go-fuse/v2/fs"
)

var previousValsIds = mapset.NewSet[string]()

// Refresh the list of vals in the filesystem
func refreshVals(ctx context.Context, root *fs.Inode, client client.Client) {
	newVals := mapset.NewSet[string]()
	myVals, err := getMyVals(ctx, client)
	log.Printf("Fetched %d vals", len(myVals))

	if err != nil {
		log.Printf(err.Error())
		return
	}

	// Convert the looked up list to a set
	for _, val := range myVals {
		newVals.Add(val.GetId())
	}

	// If the set of vals hasn't changed, don't do anything
	if newVals.Equal(previousValsIds) {
		return
	}

	// Remove vals that are no longer in the set
	previousVals := previousValsIds.Clone()
	valsDelta := newVals.Difference(previousVals)
	removedCount := 0
	for valID := range valsDelta.Iter() {
		// Fetch the current list of vals
		val, resp, err := client.APIClient.ValsAPI.ValsGet(context.Background(), valID).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			// Failed to fetch the val
			log.Printf(err.Error())
			return
		}

		// The val file no longer exists online, so remove it
		valName := valfile.ConstructFilename(val.GetName(), valfile.ValType(val.GetType()))
		root.RmChild(valName)
		removedCount += 1
	}
	log.Printf("Removed %d vals", removedCount)

	// Add vals that are in the set but not in the previous set
	myValsSetClone := newVals.Clone()
	myValsSetClone.Difference(previousValsIds)
	addedCount := 0
	for valID := range myValsSetClone.Iter() {
		extVal, _, err := client.APIClient.ValsAPI.ValsGet(ctx, valID).Execute()
		if err != nil {
			log.Printf(err.Error())
			return
		}

		valFile, err := valfile.NewValFile(*extVal, &client)
		if err != nil {
			log.Fatal("Error creating val file", err)
		}

		// Create a new inode for the val file and add it to the folder
		root.NewPersistentInode(
			ctx,
			valFile,
			fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})
		filename := valfile.ConstructFilename(extVal.Name, valfile.ValType(extVal.Type))
		added := root.AddChild(filename, &valFile.Inode, true)

		if !added {
			log.Printf("Failed to add val %s", extVal.GetId())
			return
		}
		addedCount += 1
	}
	log.Printf("Added %d vals", addedCount)

	// Update the previousVals set
	previousValsIds = newVals
}

const lookupCap = 99

// Get a list of all the vals belonging to the authed user
func getMyVals(ctx context.Context, client client.Client) ([]valgo.ExtendedVal, error) {
	log.Println("Fetching all of my vals")

	// Fetch my ID
	meResp, httpResp, err := client.APIClient.MeAPI.MeGet(context.Background()).Execute()
	if err != nil || httpResp.StatusCode != http.StatusOK {
		log.Printf(err.Error())
		return nil, err
	}

	// Use my ID to fetch my vals
	offset := 0
	allVals := []valgo.ExtendedVal{}
	for {
		// Request the next batch of vals
		vals, resp, err := client.APIClient.UsersAPI.UsersVals(ctx, meResp.GetId()).Offset(int32(offset)).Limit(99).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			return nil, err
		}

		// Update the list of vals
		for _, val := range vals.Data {
			// Fetch the full data for the val
			extVal, resp, err := client.APIClient.ValsAPI.ValsGet(ctx, val.Id).Execute()
			if err != nil || resp.StatusCode != http.StatusOK {
				return nil, err
			}
			allVals = append(allVals, *extVal)
		}

		// Check to see if we have reached the end
		if len(vals.Data) < lookupCap {
			break
		}

		offset += lookupCap
	}
	log.Printf("Fetched all of my vals. Found %d vals", len(allVals))

	return allVals, nil
}
