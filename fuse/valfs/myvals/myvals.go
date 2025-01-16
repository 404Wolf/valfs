package fuse

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"syscall"
	"time"

	_ "embed"

	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	common "github.com/404wolf/valfs/common"
	valfile "github.com/404wolf/valfs/fuse/valfile"
)

// The folder with all of my vals in it
type MyVals struct {
	fs.Inode
	client *common.Client
}

// Set up background refresh of vals and retreive an auto updating folder of
// val files
func NewMyVals(parent *fs.Inode, client *common.Client, ctx context.Context) *MyVals {
	myValsDir := &MyVals{
		client: client,
	}
	attrs := fs.StableAttr{Mode: syscall.S_IFDIR}
	parent.NewPersistentInode(ctx, myValsDir, attrs)

	refreshVals(ctx, &myValsDir.Inode, *client)
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			refreshVals(ctx, &myValsDir.Inode, *client)
			log.Println("Refreshed vals")
		}
	}()

	return myValsDir
}

var _ = (fs.NodeUnlinker)((*MyVals)(nil))

// Handle deletion of a file by also deleting the val
func (c *MyVals) Unlink(ctx context.Context, name string) syscall.Errno {
	log.Printf("Deleting val %s", name)
	child := c.GetChild(name)
	if child == nil {
		return syscall.ENOENT
	}

	// Cast the file handle to a ValFile
	valFile, ok := child.Operations().(*valfile.ValFile)
	if !ok {
		return syscall.EINVAL
	}

	_, err := c.client.APIClient.ValsAPI.ValsDelete(ctx, valFile.BasicData.Id).Execute()
	if err != nil {
		log.Printf("Error deleting val %s: %v", valFile.BasicData.Id, err)
		return syscall.EIO
	}
	log.Printf("Deleted val %s", valFile.BasicData.Id)

	return 0
}

var _ = (fs.NodeCreater)((*MyVals)(nil))

func guessFilename(guess string) (hopeless bool, valName *string, valType *valfile.ValType) {
	// Parse the filename of the val
	valNameAttempt, valTypeAttempt := valfile.ExtractFromFilename(guess)

	// Try to guess the type if it is unknown
	if valTypeAttempt == valfile.Unknown {
		re := regexp.MustCompile(`^([^\.]+\.?)+\.tsx?`)
		if re.MatchString(guess) {
			valName = &re.FindStringSubmatch(guess)[1]
			valTypeRef := valfile.DefaultType
			return false, valName, &valTypeRef
		} else {
			return true, nil, nil
		}
	} else {
		return false, &valNameAttempt, &valTypeAttempt
	}
}

// Create a new val on new file creation
func (c *MyVals) Create(
	ctx context.Context,
	name string,
	flags uint32,
	mode uint32,
	entryOut *fuse.EntryOut,
) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, code syscall.Errno) {
	hopeless, valName, valType := guessFilename(name)
	if hopeless {
		return nil, nil, 0, syscall.EINVAL
	}
	log.Printf("Creating val %s of type %s", *valName, *valType)

	// Make a request to create the val
	templateCode := valfile.GetTemplate(*valType)
	createReq := valgo.NewValsCreateRequest(string(templateCode))
	createReq.SetName(*valName)
	createReq.SetType(string(*valType))
	createReq.SetPrivacy(valfile.DefaultPrivacy)
	val, resp, err := c.client.APIClient.ValsAPI.ValsCreate(ctx).ValsCreateRequest(*createReq).Execute()

	// Check if the request was successful
	if err != nil {
		log.Printf("Error creating val: %v", err)
		return nil, nil, 0, syscall.EIO
	} else if resp.StatusCode != 201 {
		log.Printf("Error creating val: %v", resp)
		return nil, nil, 0, syscall.EIO
	} else {
		log.Printf("Created val %v", val)
	}
	log.Printf("Created val %s", val.Id)

	// Create a val file that we can hand over
	valFile, err := valfile.NewValFileFromExtendedVal(*val, c.client)
	if err != nil {
		log.Fatal("Error creating val file", err)
	}
	c.NewPersistentInode(
		ctx,
		valFile,
		fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})

	// Afterwards move the file
	filename := valfile.ConstructFilename(val.Name, valfile.ValType(val.Type))
	if filename != name {
		go func() {
			c.MvChild(name, &c.Inode, filename, true)
		}()
	}

	// Open the file handle
	fileHandle, _, _ := valFile.Open(ctx, flags)
	valFile.ModifiedNow()

	// Create a file handle
	return &valFile.Inode, &fileHandle, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

var _ = (fs.NodeRenamer)((*MyVals)(nil))

// Rename a val, and change the name in valtown
func (c *MyVals) Rename(
	ctx context.Context,
	oldName string,
	newParent fs.InodeEmbedder,
	newName string,
	code uint32,
) syscall.Errno {
	// Check if we're moving between directories
	if newParent.EmbeddedInode().StableAttr().Ino != c.Inode.StableAttr().Ino {
		log.Printf("Cannot move val out of the `myvals` directory")
		return syscall.EINVAL
	}

	// Validate the new filename
	hopeless, valName, valType := guessFilename(newName)
	if hopeless {
		return syscall.EINVAL
	}

	// Check if the new filename already exists
	if c.GetChild(newName) != nil {
		return syscall.EEXIST
	}

	inode := c.GetChild(oldName)
	if inode == nil {
		return syscall.ENOENT
	}
	valFile := inode.Operations().(*valfile.ValFile)

	// Prepare the update request
	valUpdateReq := valgo.NewValsUpdateRequest()
	valUpdateReq.SetName(*valName)
	valUpdateReq.SetType(string(*valType))

	// Update the val in the backend
	resp, err := c.client.APIClient.ValsAPI.ValsUpdate(ctx, valFile.BasicData.Id).ValsUpdateRequest(*valUpdateReq).Execute()
	if err != nil || resp.StatusCode != http.StatusNoContent {
		log.Printf("Error updating val %s: %v", oldName, err)
		return syscall.EIO
	}

	// Perform the rename in the filesystem
	newValidatedFilename := valfile.ConstructFilename(*valName, *valType)
	c.Inode.ExchangeChild(oldName, c.EmbeddedInode(), newValidatedFilename)

	// Fetch what the change produced
	extVal, resp, err := c.client.APIClient.ValsAPI.ValsGet(ctx, valFile.BasicData.Id).Execute()
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Error fetching val. Err %v: Resp: %v", err, resp)
		return syscall.EIO
	}
	valFile.ExtendedData = extVal
	valFile.BasicData.Name = *valName
	valFile.BasicData.Type = string(*valType)

	return syscall.F_OK
}

var _ = (fs.NodeReaddirer)((*MyVals)(nil))

// List the contents of the directory
func (c *MyVals) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Printf("Listing %d val files", len(c.Children()))

	entries := []fuse.DirEntry{}
	for _, valFileInode := range c.Children() {
		valFile := valFileInode.Operations().(*valfile.ValFile)
		filename := valfile.ConstructFilename(valFile.BasicData.Name, valfile.ValType(valFile.BasicData.Type))

		entries = append(entries, fuse.DirEntry{
			Mode: syscall.S_IFREG,
			Name: filename,
			Ino:  valFile.Inode.StableAttr().Ino,
			Off:  0,
		})
	}

	return fs.NewListDirStream(entries), 0
}

var _ = (fs.NodeLookuper)((*MyVals)(nil))

// Fetch an inode
func (c *MyVals) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child := c.GetChild(name)
	if child == nil {
		return nil, syscall.ENOENT
	}

	valFile, ok := child.Operations().(*valfile.ValFile)
	if !ok {
		return nil, syscall.EINVAL
	}

	return &valFile.Inode, 0
}
