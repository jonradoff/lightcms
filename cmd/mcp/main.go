// Command lightcms-mcp runs the LightCMS MCP server over stdio, exposing content management tools to MCP clients.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jonradoff/lightcms/v6/internal/apiclient"
	"github.com/jonradoff/lightcms/v6/internal/mcp"
)

func main() {
	// Read configuration from environment variables
	baseURL := os.Getenv("LIGHTCMS_URL")
	apiKey := os.Getenv("LIGHTCMS_API_KEY")

	if baseURL == "" {
		// Default to localhost
		baseURL = "http://localhost:8082"
	}

	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "Error: LIGHTCMS_API_KEY environment variable is required\n")
		fmt.Fprintf(os.Stderr, "Create an API key in the admin panel at /cm/api-keys\n")
		os.Exit(1)
	}

	// Create API client
	client := apiclient.New(baseURL, apiKey)

	// Create and run MCP server
	ctx := context.Background()
	server := mcp.NewServer(client)
	if err := server.Run(ctx); err != nil {
		log.Printf("MCP server error: %v", err)
		os.Exit(1)
	}
}
