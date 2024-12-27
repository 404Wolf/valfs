package fuse

import (
	"log"
	"os"
)

// Get the path to the current program's binary file
func getCurrentExecutablePath() string {
	execPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	return execPath
}

// Prepend a shebang that defines how to execute it
func AffixShebang(code string) string {
	return "#!" + getCurrentExecutablePath() + "\n\n" + code
}
