package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/talosprotocol/talos-sdk-go/pkg/talos/mcp"
)

func main() {
	baseURL := os.Getenv("GATEWAY_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8001"
	}

	apiKey := os.Getenv("TALOS_API_KEY")
	if apiKey == "" {
		apiKey = "sk-test-key-1"
	}

	client := mcp.NewClient(baseURL, apiKey)

	fmt.Printf("Listing servers from %s...\n", baseURL)
	servers, err := client.ListServers(context.Background())
	if err != nil {
		log.Fatalf("Failed to list servers: %v", err)
	}

	fmt.Printf("Found %d servers:\n", len(servers))
	for _, s := range servers {
		fmt.Printf("- %s (%s)\n", s.ID, s.Name)

		tools, err := client.ListTools(context.Background(), s.ID)
		if err != nil {
			fmt.Printf("  Error listing tools: %v\n", err)
			continue
		}
		fmt.Printf("  Tools (%d):\n", len(tools))
		for _, t := range tools {
			fmt.Printf("    * %s: %s\n", t.Name, t.Description)
		}
	}

	fmt.Println("\nGo SDK Interop Verification Successful!")
}
