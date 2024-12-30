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

	valfile "github.com/404wolf/valfs/fuse/valfile"
)

// A container for a val file system, with metadata about the file system
type valFSState struct {
	IdToVal   map[string](*valgo.ExtendedVal)
	NameToVal map[string](*valgo.ExtendedVal)
}

type ValFS struct {
	fs.Inode
	MineDir   *MineDir
	APIClient *valgo.APIClient
	State     valFSState
	MountDir  string
}

type MineDir struct {
	fs.Inode
	ValFS *ValFS
}

// Mount the filesystem
func (c *ValFS) Mount() error {
	c.Inode = fs.Inode{}

	server, err := fs.Mount(c.MountDir, c, &fs.Options{
		MountOptions: fuse.MountOptions{
			Debug: false,
		},
		OnAdd: func(ctx context.Context) {
			// Create the "mine" folder
			mineDir := MineDir{Inode: fs.Inode{}, ValFS: c}
			mineDirInode := c.NewPersistentInode(ctx, &mineDir.Inode, fs.StableAttr{Ino: 0, Mode: syscall.S_IFDIR})
			c.AddChild("mine", mineDirInode, true)
			c.MineDir = &mineDir

			c.AddDenoJSON(ctx)                 // Add the deno.json file
			c.refreshVals(ctx, &mineDir.Inode) // Initial load
			ticker := time.NewTicker(5 * time.Second)
			go func() {
				for range ticker.C {
					c.refreshVals(ctx, &mineDir.Inode)
				}
			}()
		},
	})

	if err != nil {
		log.Fatal(err)
		return err
	}

	log.Printf("Unmount by calling 'umount %s'\n", c.MountDir)
	server.Wait()
	return nil
}

// Create a new ValFile object and corresponding inode from a basic val instance
func (c *ValFS) ValToValFile(
	ctx context.Context,
	val valgo.ExtendedVal,
) *valfile.ValFile {
	valFile, err := valfile.NewValFile(val, c.APIClient)

	if err != nil {
		log.Fatal("Error creating val file", err)
	}

	c.Inode.NewPersistentInode(
		ctx,
		valFile,
		fs.StableAttr{Mode: syscall.S_IFREG, Ino: 0})

	return valFile
}

var _ = (fs.NodeUnlinker)((*MineDir)(nil))

// Handle deletion of a file by also deleting the val
func (c *MineDir) Unlink(ctx context.Context, name string) syscall.Errno {
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
	_, err := c.ValFS.APIClient.ValsAPI.ValsDelete(ctx, valFile.ValData.Id).Execute()
	if err != nil {
		log.Printf("Error deleting val %s: %v", valFile.ValData.Id, err)
		return syscall.EIO
	}
	log.Printf("Deleted val %s", valFile.ValData.Id)

	return 0
}

var _ = (fs.NodeCreater)((*MineDir)(nil))

//go:embed deno.json
var DefaultValContents []byte

const DefaultValType = valfile.Script

// Create a new val on new file creation
func (c *MineDir) Create(
	context context.Context,
	name string,
	flags uint32,
	mode uint32,
	entryOut *fuse.EntryOut,
) (inode *fs.Inode, fs fs.FileHandle, fuseFlags uint32, error syscall.Errno) {
	// Parse the filename of the val
	valName, valType := valfile.ExtractFromFilename(name)

	log.Printf("Creating val %s of type %s", valName, valType)
	createReq := valgo.NewValsCreateRequest(string(DefaultValContents))
	createReq.Name = &valName
	valTypeStr := string(valType)
	createReq.Type = &valTypeStr
	valPrivacyStr := "private"
	createReq.Privacy = &valPrivacyStr
	val, _, err := c.ValFS.APIClient.ValsAPI.ValsCreate(context).ValsCreateRequest(*createReq).Execute()
	log.Printf("Created val %s", val.Id)

	if err != nil {
		log.Printf("Error creating val: %v", err)
		return nil, nil, 0, syscall.EIO
	}

	// Add the val as a file
	valFile := c.ValFS.ValToValFile(context, *val)
	filename := valfile.ConstructFilename(val.Name, valfile.ValType(val.Type))
	c.AddChild(filename, &valFile.Inode, true)
	log.Printf("Added val %s to mine", val.Id)

	return &valFile.Inode, &valfile.ValFileHandle{}, fuse.FOPEN_DIRECT_IO, syscall.F_OK
}

var _ = (fs.NodeCreater)((*ValFS)(nil))

// Create a new val on new file creation
func (c *ValFS) Create(
	context context.Context,
	name string,
	flags uint32,
	mode uint32,
	entryOut *fuse.EntryOut,
) (inode *fs.Inode, fs fs.FileHandle, fuseFlags uint32, error syscall.Errno) {
	log.Printf("Creating val %s of type %s", name, DefaultValType)
	return c.MineDir.Create(context, name, flags, mode, entryOut)
}
