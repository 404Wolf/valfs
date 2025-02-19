package valfs

import (
	"context"
	"syscall"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
)

// previousValIds maintains a mapping of val IDs to their corresponding ValFile objects
// This allows us to track which vals are currently mounted in the filesystem
var previousValIds = make(map[string]*ValFile)

// refreshVals updates the filesystem to reflect the current state of vals in ValTown.
// It performs three main operations:
// 1. Adds new vals that weren't previously mounted
// 2. Updates existing vals that have new versions
// 3. Removes vals that no longer exist in ValTown
func refreshVals(ctx context.Context, root *fs.Inode, client common.Client) error {
	common.Logger.Info("Starting vals refresh operation")

	// Fetch current vals from ValTown
	newVals, err := getMyVals(ctx, client)
	if err != nil {
		common.Logger.Errorf("Failed to fetch vals from ValTown: %v", err)
		return err
	}

	// Create a map for quick lookups of new vals by their IDs
	newValsIdsToBasicVals := make(map[string]valgo.BasicVal)
	for _, newVal := range newVals {
		newValsIdsToBasicVals[newVal.GetId()] = newVal
	}
	common.Logger.Infof("Fetched %d vals from ValTown", len(newVals))

	// Process each new val
	for _, newVal := range newVals {
		prevValFile, exists := previousValIds[newVal.GetId()]

		// Handle new vals that need to be added
		if !exists {
			common.Logger.Debugf("Processing new val: %s (ID: %s)", newVal.GetName(), newVal.GetId())
			valFile, err := NewValFileFromBasicVal(ctx, newVal, &client)
			if err != nil {
				common.Logger.Errorf("Failed to create val file for %s: %v", newVal.GetId(), err)
				return err
			}
			filename := ConstructFilename(newVal.GetName(), ValType(newVal.GetType()))
			root.NewPersistentInode(ctx, valFile, fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})
			root.AddChild(filename, &valFile.Inode, true)
			previousValIds[newVal.GetId()] = valFile
			common.Logger.Infof("Added new val to filesystem: %s (ID: %s)", filename, newVal.GetId())

			// Cache the new val
			common.DenoCache(newVal.GetName(), &client)
		}

		// Update existing vals if they have a newer version
		if exists && newVal.GetVersion() > prevValFile.BasicData.GetVersion() {
			common.Logger.Debugf("Updating existing val %s from version %d to %d",
				newVal.GetId(), prevValFile.BasicData.GetVersion(), newVal.GetVersion())
			prevValFile.BasicData = newVal
			prevValFile.ModifiedNow()
			prevValFile.EmbeddedInode().Root().NotifyContent(0, 0)
			common.Logger.Infof("Updated val %s to version %d", newVal.GetId(), newVal.GetVersion())

			// Cache the updated val
			common.DenoCache(newVal.GetName(), &client)
		}
	}

	// Remove vals that no longer exist in ValTown
	for _, oldVal := range previousValIds {
		if _, exists := newValsIdsToBasicVals[oldVal.BasicData.GetId()]; !exists {
			filename := ConstructFilename(oldVal.BasicData.GetName(), ValType(oldVal.BasicData.GetType()))
			common.Logger.Debugf("Removing val %s (ID: %s) as it no longer exists in ValTown",
				filename, oldVal.BasicData.GetId())
			root.RmChild(filename)
			delete(previousValIds, oldVal.BasicData.GetId())
			common.Logger.Infof("Removed val %s from filesystem", filename)
		}
	}

	common.Logger.Info("Vals refresh operation completed successfully")
	return nil
}

// lookupCap defines the maximum number of vals to fetch in a single API request
const lookupCap = 99

// getMyVals retrieves all vals belonging to the authenticated user from ValTown.
// It handles pagination automatically, fetching all vals in batches of lookupCap size.
func getMyVals(ctx context.Context, client common.Client) ([]valgo.BasicVal, error) {
	common.Logger.Info("Beginning fetch of all user vals")

	// Fetch the authenticated user's ID
	meResp, _, err := client.APIClient.MeAPI.MeGet(context.Background()).Execute()
	if err != nil {
		common.Logger.Errorf("Failed to fetch user information: %v", err)
		return nil, err
	}
	common.Logger.Debugf("Retrieved user ID: %s", meResp.GetId())

	// Fetch vals using pagination
	offset := 0
	allVals := []valgo.BasicVal{}
	for {
		common.Logger.Debugf("Fetching vals batch with offset %d", offset)
		request := client.APIClient.UsersAPI.UsersVals(ctx, meResp.GetId())
		request = request.Offset(int32(offset))
		request = request.Limit(99)
		vals, _, err := request.Execute()
		if err != nil {
			common.Logger.Errorf("Failed to fetch vals batch at offset %d: %v", offset, err)
			return nil, err
		}

		allVals = append(allVals, vals.Data...)
		common.Logger.Debugf("Retrieved %d vals in current batch", len(vals.Data))

		// Break if we've received fewer vals than the maximum limit
		if len(vals.Data) < lookupCap {
			break
		}

		offset += lookupCap
	}

	common.Logger.Infof("Successfully retrieved all vals. Total count: %d", len(allVals))
	return allVals, nil
}
