package fuse

import (
	"context"
	"log"
	"syscall"
	"time"

	"github.com/404wolf/valgo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// A container for a val file system, with metadata about the file system
type ValFS struct {
	ValClient *valgo.APIClient
	MountDir  string
}

// Create a new val file system instance
func NewValFS(valTownClient *valgo.APIClient, mountDir string) *ValFS {
	return &ValFS{
		ValClient: valTownClient,
		MountDir:  mountDir,
	}
}

// Mount the filesystem
func (c *ValFS) Mount() error {
	root := &fs.Inode{}
	log.Println("Mounted root Inode")

	server, err := fs.Mount(c.MountDir, root, &fs.Options{
		MountOptions: fuse.MountOptions{Debug: false},
		OnAdd: func(ctx context.Context) {
			c.refreshVals(ctx, root) // Initial load
			ticker := time.NewTicker(5 * time.Second)
			go func() {
				for range ticker.C {
					c.refreshVals(ctx, root)
				}
			}()
		},
	})

	if err != nil {
		log.Fatal(err)
		return err
	}

	log.Printf("Unmount by calling 'fusermount -u %s'\n", c.MountDir)
	server.Wait()
	return nil
}

func (c *ValFS) addValFile(ctx context.Context, root *fs.Inode, val valgo.BasicVal) {
	valFile := &valFile{
		ValData:   val,
		ValClient: c.ValClient,
	}

	ch := root.NewPersistentInode(
		ctx,
		valFile,
		fs.StableAttr{Mode: syscall.S_IFREG}) // regular file

	root.AddChild(val.Name+".tsx", ch, true)
}

func (c *ValFS) removeValFile(_ context.Context, root *fs.Inode, val valgo.BasicVal) {
	root.RmChild(val.Name + ".tsx")
}

var previousVals = mapset.NewSet[string]()

// Refresh the list of vals in the filesystem
func (c *ValFS) refreshVals(ctx context.Context, root *fs.Inode) {
	// Fetch my ID
	me_resp, _, err := c.ValClient.MeAPI.MeGet(context.Background()).Execute()
	if err != nil {
		log.Fatal(err)
	}

	// Use my ID to fetch my vals
	resp, _, err := c.ValClient.UsersAPI.UsersVals(ctx, me_resp.GetId()).Offset(0).Limit(50).Execute()
	if err != nil {
		log.Fatal(err)
	}

	// Set of the IDs of the current vals
	myValsSet := mapset.NewSet[string]()
	for _, val := range resp.GetData() {
		myValsSet.Add(val.Id)
	}

	// If the set of vals hasn't changed, don't do anything
	if myValsSet.Equal(previousVals) {
		return
	}

	// Remove vals that are no longer in the set
	previousValsClone := previousVals.Clone()
	previousValsClone.Difference(myValsSet)
	for valID := range previousValsClone.Iter() {
		resp, _, err := c.ValClient.ValsAPI.ValsGet(context.Background(), valID).Execute()
		if err != nil {
			log.Fatal(err)
		}

		val := resp.ToBasicVal()
		c.removeValFile(ctx, root, val)
	}

	// Add vals that are in the set but not in the previous set
	myValsSetClone := myValsSet.Clone()
	myValsSetClone.Difference(previousVals)
	for valID := range myValsSetClone.Iter() {
		resp, _, err := c.ValClient.ValsAPI.ValsGet(ctx, valID).Execute()
		if err != nil {
			log.Fatal(err)
		}
		val := resp.ToBasicVal()
		c.addValFile(ctx, root, val)
	}

	// Update the previousVals set
	previousVals = myValsSet
}
