package common

type ValfsConfig struct {
	// A val town admin API key
	APIKey string

	// The root directory of the Valfs filesystem.
	MountPoint string

	// Automatically cache required deno packages
	DenoCache bool

	// Add a deno.json for editing with a deno supported IDE/LSP (e.g. denols)
	DenoJson bool

	// Automatically refresh content using the api with polling
	AutoRefresh bool

	// Automatically unmount the directory you mounted to on exit
	AutoUnmountOnExit bool

	// How often to poll val town website for changes
	AutoRefreshInterval int

	// Add a directory for your vals
	EnableValsDirectory bool

	// Whether to enable go fuse's debug mode
	GoFuseDebug bool
}
