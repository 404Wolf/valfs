package main

import (
	"fmt"
	"github.com/404wolf/valfs/cmd"
	"github.com/404wolf/valfs/sdk"
	"github.com/joho/godotenv"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func loadEnvFile() error {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	value := os.Getenv("VALTOWN_API_KEY")
	if value == "" {
		fmt.Println("VALTOWN_API_KEY is not set")
	}
	return err
}

func setup() {
	loadEnvFile()

	tempFile, err := os.CreateTemp("", "run-val")
	if err != nil {
		log.Fatal(err)
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
}

func execute() {
	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func isShebangCall() bool {
	return len(os.Args) > 1 && strings.HasSuffix(os.Args[1], ".tsx")
}

func handleShebangCall(args []string) {
	log.SetOutput(io.Discard)

	scriptPath := args[0]
	basePath := filepath.Base(scriptPath)
	fmt.Printf("Running %s...\n", basePath)

	client, err := sdk.NewValTownClient()
	if err != nil {
		log.Fatal(err)
	}
	client.Vals.Run(scriptPath)
}

func main() {
	setup()

	// If it is a shebang call, manually trigger the correct command
	if isShebangCall() {
		handleShebangCall(os.Args[1:])
		return
	} else {
		// Otherwise, proceed as normal with cobra
		execute()
	}
}
