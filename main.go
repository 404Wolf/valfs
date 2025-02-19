package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/404wolf/valfs/cmd"
	"github.com/404wolf/valfs/common"
	"go.uber.org/zap"
)

func isShebangCall() bool {
	return len(os.Args) > 1 && strings.HasSuffix(os.Args[1], ".tsx")
}

func main() {
	common.Logger = zap.NewNop().Sugar()

	if isShebangCall() {
		fmt.Println("Shebang call detected. Not supported yet.")
		return
	} else {
		err := cmd.Execute()

		if err != nil {
			os.Exit(1)
		}
	}
}
