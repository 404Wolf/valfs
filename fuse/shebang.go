package fuse

import (
	"log"
	"os"
)

func getCurrentExecutablePath() string {
	execPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	return execPath
}

// Prepend a shebang that defines how to execute it
func executeValShebang(code string) string {
	return "#!" + getCurrentExecutablePath() + "\n\n" + code
}
