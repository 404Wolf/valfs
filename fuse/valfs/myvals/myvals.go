package fuse

import (
	"context"
	"log"
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

	ticker := time.NewTicker(5 * time.Second)
	refreshVals(ctx, &myValsDir.Inode, *client)
	go func() {
		for range ticker.C {
			refreshVals(ctx, parent, *client)
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

	log.Printf("Deleting val %s", valFile.ExtendedData.Id)
	_, err := c.client.APIClient.ValsAPI.ValsDelete(ctx, valFile.ExtendedData.Id).Execute()
	if err != nil {
		log.Printf("Error deleting val %s: %v", valFile.ExtendedData.Id, err)
		return syscall.EIO
	}
	log.Printf("Deleted val %s", valFile.ExtendedData.Id)

	return 0
}

var _ = (fs.NodeCreater)((*MyVals)(nil))

// Create a new val on new file creation
func (c *MyVals) Create(
	ctx context.Context,
	name string,
	flags uint32,
	mode uint32,
	entryOut *fuse.EntryOut,
) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, code syscall.Errno) {
	// Parse the filename of the val
	valName, valType := valfile.ExtractFromFilename(name)

	// Try to guess the type if it is unknown
	if valType == valfile.Unknown {
		re := regexp.MustCompile(`^([^\.]+\.?)+\.tsx?`)
		if re.MatchString(name) {
			valName = re.FindStringSubmatch(name)[1]
			valType = valfile.DefaultType
		} else {
			return nil, nil, 0, syscall.EINVAL
		}
	}

	log.Printf("Creating val %s of type %s", valName, valType)

	// Make a request to create the val
	templateCode := valfile.GetTemplate(valType)
	createReq := valgo.NewValsCreateRequest(string(templateCode))
	createReq.SetName(valName)
	createReq.SetType(string(valType))
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
	valFile, err := valfile.NewValFileFromExtended(*val, c.client)
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
