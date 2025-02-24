package valfs

import (
	"context"
	"syscall"

	_ "embed"

	common "github.com/404wolf/valfs/common"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const StaticEditorFilesPerms = 0644

type EditorFiles struct {
	fs.Inode
	client *common.Client
}

// EditorFilesConfig specifies which editor files to include
type EditorFilesConfig struct {
	IncludeCursorRules bool
	IncludeDenoJSON    bool
}

// EditorConfigInodes holds the inodes for editor configuration files
type EditorConfigInodes struct {
	CursorRules *fs.Inode
	DenoJSON    *fs.Inode
}

func addStaticEditorFile(
	parent *fs.Inode,
	ctx context.Context,
	fileContent []byte,
) *fs.Inode {
	staticEditorFile := &fs.MemRegularFile{
		Data: fileContent,
		Attr: fuse.Attr{Mode: StaticEditorFilesPerms},
	}
	return parent.NewPersistentInode(
		ctx,
		staticEditorFile,
		fs.StableAttr{Mode: syscall.S_IFREG},
	)
}

//go:embed .cursorrules
var cursorRulesFile []byte

//go:embed deno.json
var denoJSONFile []byte

func NewEditorFiles(
	parent *fs.Inode,
	client *common.Client,
	ctx context.Context,
	config EditorFilesConfig,
) EditorConfigInodes {
	inodes := EditorConfigInodes{}

	if config.IncludeCursorRules {
		common.Logger.Info("Adding .cursorrules to valfs")
		inodes.CursorRules = addStaticEditorFile(parent, ctx, cursorRulesFile)
	}

	if config.IncludeDenoJSON {
		common.Logger.Info("Adding deno.json to valfs")
		inodes.DenoJSON = addStaticEditorFile(parent, ctx, denoJSONFile)
	}

	return inodes
}
