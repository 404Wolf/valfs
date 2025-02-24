package common

type ValfsConfig struct {
	// A val town admin API key
	APIKey string

	// The root directory of the Valfs filesystem.
	MountPoint string

	// Automatically cache required deno packages
	DenoCache bool

	// Add a deno.json for editing with a deno supported IDE/LSP (e.g. denols)
	IncludeDenoJson bool

	// Add a .cursorrules to help guide cursor IDE LLMs on how to edit files
	IncludeCursorRules bool

	// Automatically refresh content using the api with polling
	AutoRefresh bool

	// Automatically unmount the directory you mounted to on exit
	AutoUnmountOnExit bool

	// How often to poll val town website for changes
	AutoRefreshInterval int

	// Add a directory for your vals
	EnableValsDirectory bool

	// Add a directory for your blob
	EnableBlobsDirectory bool

	// Whether to enable go fuse's debug mode
	GoFuseDebug bool

	// Whether to only show file metadata that is static (omit versions like ?v=
	// in urls, or the version field, which change on writes)
	StaticMeta bool

	// Whether to have vals be executable so that you can "run" them
	ExecutableVals bool
}
