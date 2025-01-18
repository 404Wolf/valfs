package main

import (
	"fmt"
	"github.com/404wolf/valfs/cmd"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strings"
)

func loadEnvFile() error {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	value := os.Getenv("VAL_TOWN_API_KEY")
	if value == "" {
		fmt.Println("VAL_TOWN_API_KEY is not set")
	}
	return err
}

func setup() {
	loadEnvFile()
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

func main() {
	setup()

	if isShebangCall() {
		fmt.Println("Shebang call detected. Not supported yet.")
		return
	} else {
		execute()
	}
}
