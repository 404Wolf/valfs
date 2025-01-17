package fuse

import (
	"context"
	"log"
	"net/http"
	"syscall"

	common "github.com/404wolf/valfs/common"
	valfile "github.com/404wolf/valfs/fuse/valfs/myvals/valfile"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
)

var previousValIds = make(map[string]*valfile.ValFile)

// Refresh the list of vals in the filesystem
func refreshVals(ctx context.Context, root *fs.Inode, client common.Client) {
	newVals, err := getMyVals(ctx, client)
	newValsIdsToBasicVals := make(map[string]valgo.BasicVal)
	for _, newVal := range newVals {
		newValsIdsToBasicVals[newVal.GetId()] = newVal
	}

	log.Printf("Fetched %d vals", len(newVals))

	if err != nil {
		log.Printf(err.Error())
		return
	}

	for _, newVal := range newVals {
		prevValFile, exists := previousValIds[newVal.GetId()]

		if !exists {
			valFile, err := valfile.NewValFileFromBasicVal(ctx, newVal, &client)
			if err != nil {
				log.Fatal("Error creating val file", err)
			}
			filename := valfile.ConstructFilename(newVal.GetName(), valfile.ValType(newVal.GetType()))
			root.NewPersistentInode(ctx, valFile, fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})
			root.AddChild(filename, &valFile.Inode, true)
			previousValIds[newVal.GetId()] = valFile

			log.Printf("Added val %s, found fresh on valtown", newVal.GetId())
		}

		// If the val already existed in our collection but is newer then update it in-place
		if exists && newVal.GetVersion() > prevValFile.BasicData.GetVersion() {
			prevValFile.BasicData = newVal
			prevValFile.ModifiedNow()
			prevValFile.EmbeddedInode().Root().NotifyContent(0, 0)
			log.Printf("Updated val %s, found newer on valtown", newVal.GetId())
		}
	}

	// For each old val, if it is not in new vals remove it
	for _, oldVal := range previousValIds {
		if _, exists := newValsIdsToBasicVals[oldVal.BasicData.GetId()]; !exists {
			filename := valfile.ConstructFilename(oldVal.BasicData.GetName(), valfile.ValType(oldVal.BasicData.GetType()))
			root.RmChild(filename)
			delete(previousValIds, oldVal.BasicData.GetId())
			log.Printf("Removed val %s no longer found on valtown", oldVal.BasicData.GetId())
		}
	}
}

const lookupCap = 99

// Get a list of all the vals belonging to the authed user
func getMyVals(ctx context.Context, client common.Client) ([]valgo.BasicVal, error) {
	log.Println("Fetching all of my vals")

	// Fetch my ID
	meResp, httpResp, err := client.APIClient.MeAPI.MeGet(context.Background()).Execute()
	if err != nil || httpResp.StatusCode != http.StatusOK {
		log.Printf(err.Error())
		return nil, err
	}

	// Use my ID to fetch my vals
	offset := 0
	allVals := []valgo.BasicVal{}
	for {
		// Request the next batch of vals
		vals, resp, err := client.APIClient.UsersAPI.UsersVals(ctx, meResp.GetId()).Offset(int32(offset)).Limit(99).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			return nil, err
		}

		// Update the list of vals
		for _, val := range vals.Data {
			allVals = append(allVals, val)
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
