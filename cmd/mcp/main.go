package main

import (
	"context"
	"log"
	"os"

	"lightcms/config"
	"lightcms/internal/database"
	"lightcms/internal/mcp"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	ctx := context.Background()
	db, err := database.Connect(ctx, cfg.MongoURI, "lightcms")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Disconnect(ctx)

	// Create and run MCP server
	server := mcp.NewServer(db)
	if err := server.Run(ctx); err != nil {
		log.Printf("MCP server error: %v", err)
		os.Exit(1)
	}
}
