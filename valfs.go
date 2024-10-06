package main

import (
	"fmt"
	"github.com/404wolf/valfs/cmd"
	"github.com/joho/godotenv"
	"log"
	"os"
)

func loadEnvFile() error {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	value := os.Getenv("VALTOWN_API_KEY")
	if value == "" {
		fmt.Println("VALTOWN_API_KEY is not set")
		panic("VALTOWN_API_KEY is not set")
	}
	return err
}

func main() {
	loadEnvFile()
	cmd.Execute()
}
