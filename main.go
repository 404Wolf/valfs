package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/404wolf/valfs/cmd"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func loadEnvFile() error {
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}

	value := os.Getenv("VAL_TOWN_API_KEY")
	if value == "" {
		value = os.Getenv("VAL_TOWN_API_KEY")
		if value == "" {
			return fmt.Errorf("VAL_TOWN_API_KEY is not set in .env file or environment variables")
		}
		fmt.Println("Using VAL_TOWN_API_KEY from environment variables")
	} else {
		fmt.Println("Using VAL_TOWN_API_KEY from .env file")
	}

	return nil
}

func setupLogger() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()
	sugar.Infof("Failed to fetch URL: %s", "test")
}

func setup() {
	loadEnvFile()
	setupLogger()
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
