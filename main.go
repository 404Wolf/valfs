package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/404wolf/valfs/cmd"
	"go.uber.org/zap"
)

func SetupLogger() *zap.SugaredLogger {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()
	return sugar
}

func isShebangCall() bool {
	return len(os.Args) > 1 && strings.HasSuffix(os.Args[1], ".tsx")
}

func main() {
	logger := SetupLogger()

	if isShebangCall() {
		fmt.Println("Shebang call detected. Not supported yet.")
		return
	} else {
		err := cmd.Execute(logger)

		if err != nil {
			os.Exit(1)
		}
	}
}
