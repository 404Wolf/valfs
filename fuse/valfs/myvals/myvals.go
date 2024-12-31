package fuse

import (
	"context"
	"log"
	"syscall"
	"time"

	_ "embed"

	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	client "github.com/404wolf/valfs/client"
	valfile "github.com/404wolf/valfs/fuse/valfile"
)

// The folder with all of my vals in it
type MyVals struct {
	fs.Inode
	client *client.Client
}

// Set up background refresh of vals and retreive an auto updating folder of
// val files
func NewMyVals(parent *fs.Inode, client *client.Client, ctx context.Context) *MyVals {
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

	log.Printf("Deleting val %s", valFile.ValData.Id)
	_, err := c.client.APIClient.ValsAPI.ValsDelete(ctx, valFile.ValData.Id).Execute()
	if err != nil {
		log.Printf("Error deleting val %s: %v", valFile.ValData.Id, err)
		return syscall.EIO
	}
	log.Printf("Deleted val %s", valFile.ValData.Id)

	return 0
}

var _ = (fs.NodeCreater)((*MyVals)(nil))

//go:embed init.ts
var DefaultValContents []byte

const DefaultValType = valfile.Script

// Create a new val on new file creation
func (c *MyVals) Create(
	ctx context.Context,
	name string,
	flags uint32,
	mode uint32,
	entryOut *fuse.EntryOut,
) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, error syscall.Errno) {
	// Parse the filename of the val
	valName, valType := valfile.ExtractFromFilename(name)

	log.Printf("Creating val %s of type %s", valName, valType)
	createReq := valgo.NewValsCreateRequest(string(DefaultValContents))
	createReq.Name = &valName
	valTypeStr := string(valType)
	createReq.Type = &valTypeStr
	valPrivacyStr := "private"
	createReq.Privacy = &valPrivacyStr
	val, _, err := c.client.APIClient.ValsAPI.ValsCreate(ctx).ValsCreateRequest(*createReq).Execute()
	log.Printf("Created val %s", val.Id)

	if err != nil {
		log.Printf("Error creating val: %v", err)
		return nil, nil, 0, syscall.EIO
	}

	// Create a val file that we can hand over
	valFile, err := valfile.NewValFile(*val, c.client)
	attr := fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0}
	c.Inode.NewPersistentInode(ctx, &valFile.Inode, attr)

	if err != nil {
		log.Fatal("Error creating val file", err)
	}

	// Add the val file to the folder
	filename := valfile.ConstructFilename(val.Name, valfile.ValType(val.Type))
	c.AddChild(filename, &valFile.Inode, true)
	log.Printf("Added val %s to mine", val.Id)

	return &valFile.Inode, &valfile.ValFileHandle{}, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}
