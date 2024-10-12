package fuse

import (
	"log"
	"os"
)

// Setup a path to an executable for running vals
func setupValRunner() {
	tempFile, err := os.CreateTemp("", "run-val")
	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
}

// Prepend a shebang that defines how to execute it
func executeValShebang(code string) string {
	return "#/" + code
}
