package cmd

import (
	"encoding/json"
	"log"
)

func PrettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal struct: %v", err)
	}
	return string(b)
}
