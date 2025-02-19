package valfs

import (
	"time"

	common "github.com/404wolf/valfs/common"
)

const WaitTimeBeforeDenoCaching = 500

// Wait a little bit, then run a deno cache on the file that was modified so
// that it loads new modules. If configuration specifies that we should not
// recache than this is a noop.
func waitThenMaybeDenoCache(name string, client *common.Client) {
	if client.Config.DenoCache {
		common.Logger.Info("Waiting to deno cache soon")
		time.AfterFunc(
			WaitTimeBeforeDenoCaching*time.Millisecond,
			func() { common.DenoCache(name, client) },
		)
	} else {
		common.Logger.Info("Skipping deno cache as per config")
	}
}
