package fuse

import (
	"context"
	"log"
	"syscall"

	"github.com/404wolf/valfs/sdk"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// bytesFileHandle is a file handle that carries separate content for
// each Open call
type bytesFileHandle struct {
	content []byte
}

// bytesFileHandle allows readm
var _ = (fs.FileReader)((*bytesFileHandle)(nil))

func (fh *bytesFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	end := off + int64(len(dest))
	if end > int64(len(fh.content)) {
		end = int64(len(fh.content))
	}
	log.Printf("Reading from %d to %d", off, end)

	return fuse.ReadResultData(fh.content[off:end]), 0
}

// timeFile implements Open
var _ = (fs.NodeOpener)((*valFile)(nil))

func (f *valFile) Open(ctx context.Context, openFlags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	// disallow writes
	if fuseFlags&(syscall.O_RDWR|syscall.O_WRONLY) != 0 {
		return nil, 0, syscall.EROFS
	}

	// provide the Val's code as the data 
	fh = &bytesFileHandle{
		content: []byte(f.ValData.Code),
	}

	// Return FOPEN_DIRECT_IO so content is not cached.
	return fh, fuse.FOPEN_DIRECT_IO, 0
}

type valFile struct {
	fs.Inode
	ValData sdk.ValData
}

type ValFS struct {
	ValClient *sdk.ValTownClient
	ValAuthor *sdk.ValAuthor
	MountDir  string
}

func NewValFS(valClient *sdk.ValTownClient, valAuthor *sdk.ValAuthor, mountDir string) *ValFS {
	return &ValFS{
		ValClient: valClient,
		ValAuthor: valAuthor,
		MountDir:  mountDir,
	}
}

func (c *ValFS) Mount() error {
	root := &fs.Inode{}
	log.Println("Mounted root Inode")

	server, err := fs.Mount(c.MountDir, root, &fs.Options{
		MountOptions: fuse.MountOptions{Debug: false},

		OnAdd: func(ctx context.Context) {
			username := string(c.ValAuthor.Username)
			resp, err := c.ValClient.Vals.Search(username)
			if err != nil {
				log.Fatal(err)
				panic(err)
			}

			for _, val := range resp.Data {
				log.Printf("Adding val %s", val.Name)
				valFile := &valFile{
					ValData: val,
				}

				ch := root.NewPersistentInode(
					ctx,
					valFile,
					fs.StableAttr{Mode: syscall.S_IFREG}) // regular file
				root.AddChild(val.Name+".tsx", ch, true)
				log.Printf("Added child %s", val.Name)
			}
		},
	})

	if err != nil {
		log.Fatal(err)
		return err
	}

	log.Printf("Unmount by calling 'fusermount -u %s'\n", c.MountDir)

	// Serve the file system until unmounted w/`fusermount -u`
	server.Wait()

	return nil
}
