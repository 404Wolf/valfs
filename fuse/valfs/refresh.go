package fuse

import (
	"context"
	"log"
	"net/http"

	valfile "github.com/404wolf/valfs/fuse/valfile"
	"github.com/404wolf/valgo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/hanwen/go-fuse/v2/fs"
)

var previousValsIds = mapset.NewSet[string]()

// Refresh the list of vals in the filesystem
func (c *ValFS) refreshVals(ctx context.Context, root *fs.Inode) {
	newVals := mapset.NewSet[string]()
	myVals, err := getMyVals(ctx, *c.APIClient)

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
	for valID := range valsDelta.Iter() {
		// Fetch the current list of vals
		val, resp, err := c.APIClient.ValsAPI.ValsGet(context.Background(), valID).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			// Failed to fetch the val
			log.Printf(err.Error())
			return
		}

		// The val file no longer exists online, so remove it
		valName := valfile.ConstructFilename(val.GetName(), valfile.ValType(val.GetType()))
		root.RmChild(valName)
	}

	// Add vals that are in the set but not in the previous set
	myValsSetClone := newVals.Clone()
	myValsSetClone.Difference(previousValsIds)
	for valID := range myValsSetClone.Iter() {
		extVal, _, err := c.APIClient.ValsAPI.ValsGet(ctx, valID).Execute()
		if err != nil {
			log.Printf(err.Error())
			return
		}

		valFile := c.ValToValFile(ctx, *extVal)
		filename := valfile.ConstructFilename(extVal.Name, valfile.ValType(extVal.Type))
		added := root.AddChild(filename, &valFile.Inode, true)
		if !added {
			log.Printf("Failed to add val %s", extVal.GetId())
			return
		}
	}

	// Update the previousVals set
	previousValsIds = newVals
}

const lookupCap = 99

// Get a list of all the vals belonging to the authed user
func getMyVals(ctx context.Context, client valgo.APIClient) ([]valgo.ExtendedVal, error) {
	// Fetch my ID
	meResp, httpResp, err := client.MeAPI.MeGet(context.Background()).Execute()
	if err != nil || httpResp.StatusCode != http.StatusOK {
		log.Printf(err.Error())
		return nil, err
	}

	// Use my ID to fetch my vals
	offset := 0
	allVals := []valgo.ExtendedVal{}
	for {
		// Request the next batch of vals
		vals, resp, err := client.UsersAPI.UsersVals(ctx, meResp.GetId()).Offset(int32(offset)).Limit(99).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			return nil, err
		}

		// Update the list of vals
		for _, val := range vals.Data {
			// Fetch the full data for the val
			extVal, resp, err := client.ValsAPI.ValsGet(ctx, val.Id).Execute()
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

	return allVals, nil
}
