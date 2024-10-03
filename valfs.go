package main

import (
	// "fmt"
	// "github.com/404wolf/valfs/sdk"
	"github.com/404wolf/valfs/cmd"
	"github.com/joho/godotenv"
	"log"
)

func loadEnvFile() error {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	return err
}

func main() {
	cmd.Execute()
}
